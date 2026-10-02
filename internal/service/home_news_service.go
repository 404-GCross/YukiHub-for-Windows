package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"yukihub/internal/applog"
	"yukihub/internal/common/vo"
	"yukihub/internal/utils/apputils"
	"yukihub/internal/version"
)

// ======================== Galgame 资讯 ========================
//
// 端点与行为逐条对齐手机版 HomeActivity：
//   - 直连 NextMoe `https://api.nextmoe.dev/v2/news`（免密钥、匿名配额）；
//   - 取 6 条、带 `nsfw=true`；
//   - 磁盘缓存 2 小时 TTL，冷启动命中新鲜缓存就不发请求（省匿名配额）；
//   - 联网失败时回退过期缓存，只有「无缓存 + 失败」才向前端报「没有内容」；
//   - 请求超时（连接 6s / 读取 9s）单独识别，前端据此换文案。
//
// 与手机版的差异只有一处：手机端是客户端直连，桌面端走本地后端代理——
// 桌面 WebView 直连外部域名会撞 CORS/混合内容，后端代理同时能统一落盘缓存。

const (
	nextMoeNewsAPIURL = "https://api.nextmoe.dev/v2/news"
	// nextMoeNewsItemCount 一轮取几条（手机版 NEWS_ITEM_COUNT）。
	nextMoeNewsItemCount = 6
	// homeNewsCacheFileName 与手机版同名（手机端在 getCacheDir() 下）。
	homeNewsCacheFileName = "home_news_cache.json"
	// homeNewsCacheTTL 手机版 NEWS_CACHE_TTL_MS = 2h。
	homeNewsCacheTTL = 2 * time.Hour

	homeNewsConnectTimeout = 6 * time.Second
	homeNewsReadTimeout    = 9 * time.Second
)

// homeNewsCachePayload 是落盘的缓存结构（多余字段对齐手机版的 fetched_at/items）。
type homeNewsCachePayload struct {
	FetchedAt int64             `json:"fetched_at"`
	Items     []vo.HomeNewsItem `json:"items"`
}

// nextMoeNewsResponse 是 /v2/news 的响应信封。只声明用得到的字段。
type nextMoeNewsResponse struct {
	Items []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Summary     string `json:"summary"`
		SourceURL   string `json:"source_url"`
		PublishedAt string `json:"published_at"`
		Source      struct {
			Attribution string `json:"attribution"`
		} `json:"source"`
		Banner struct {
			URL string `json:"url"`
		} `json:"banner"`
	} `json:"items"`
}

// GetGalgameNews 取首页 Galgame 资讯。
//
// force=false 时优先用 2 小时内的缓存；force=true（前端点刷新）则强制联网。
// 联网失败会回退过期缓存（Stale=true），两者都没有时返回空 Items 而不报错——
// 首页不该因为一个资讯接口就弹错误。
func (s *HomeService) GetGalgameNews(force bool) (vo.HomeNewsResult, error) {
	s.newsMu.Lock()
	defer s.newsMu.Unlock()

	s.ensureNewsClientLocked()
	s.loadNewsCacheLocked()

	now := time.Now()
	// 注意：Go 的自动分号插入要求 `&&` 留在上一行行尾，不能放到行首。
	cacheFresh := len(s.newsItems) > 0 &&
		!s.newsFetchedAt.IsZero() &&
		now.Sub(s.newsFetchedAt) < homeNewsCacheTTL
	if !force && cacheFresh {
		return vo.HomeNewsResult{Items: s.newsItems}, nil
	}

	items, fetchErr := s.fetchGalgameNewsLocked()
	if fetchErr == nil && len(items) > 0 {
		s.newsItems = items
		s.newsFetchedAt = now
		s.writeNewsCacheLocked(items)
		return vo.HomeNewsResult{Items: items}, nil
	}

	timedOut := isHomeNewsTimeout(fetchErr)
	if fetchErr != nil {
		applog.LogWarningf(s.ctx, "Galgame 资讯：拉取失败：%v", fetchErr)
	}

	// 有旧数据就照旧展示（手机版同样是静默回退，不打扰用户）。
	if len(s.newsItems) > 0 {
		return vo.HomeNewsResult{
			Items:    s.newsItems,
			Stale:    true,
			TimedOut: timedOut,
		}, nil
	}
	return vo.HomeNewsResult{TimedOut: timedOut}, nil
}

// ensureNewsClientLocked 惰性构造 HTTP 客户端（必须持锁调用）。
func (s *HomeService) ensureNewsClientLocked() {
	if s.newsClient != nil {
		return
	}
	s.newsClient = &http.Client{
		Timeout: homeNewsConnectTimeout + homeNewsReadTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: homeNewsConnectTimeout,
			}).DialContext,
			ResponseHeaderTimeout: homeNewsReadTimeout,
			// 复用连接，降低匿名配额下的握手开销
			MaxIdleConns:        8,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: homeNewsConnectTimeout,
		},
	}
}

// fetchGalgameNewsLocked 联网拉取（必须持锁调用）。
func (s *HomeService) fetchGalgameNewsLocked() ([]vo.HomeNewsItem, error) {
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	query := url.Values{}
	query.Set("limit", strconv.Itoa(nextMoeNewsItemCount))
	query.Set("nsfw", "true")

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		nextMoeNewsAPIURL+"?"+query.Encode(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("构造资讯请求失败: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", version.UserAgent())

	response, err := s.newsClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求资讯接口失败: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("资讯接口返回 HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取资讯响应失败: %w", err)
	}

	var payload nextMoeNewsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析资讯响应失败: %w", err)
	}

	items := make([]vo.HomeNewsItem, 0, nextMoeNewsItemCount)
	for _, raw := range payload.Items {
		if len(items) >= nextMoeNewsItemCount {
			break
		}
		title := strings.TrimSpace(raw.Title)
		if title == "" {
			// 没标题的条目没法点，跳过（手机版同样跳过）
			continue
		}
		items = append(items, vo.HomeNewsItem{
			ID:          strings.TrimSpace(raw.ID),
			Title:       title,
			Summary:     strings.TrimSpace(raw.Summary),
			BannerURL:   strings.TrimSpace(raw.Banner.URL),
			SourceURL:   strings.TrimSpace(raw.SourceURL),
			Attribution: strings.TrimSpace(raw.Source.Attribution),
			PublishedAt: strings.TrimSpace(raw.PublishedAt),
		})
	}
	return items, nil
}

// loadNewsCacheLocked 首次调用时从磁盘载入缓存（必须持锁调用）。
//
// 与手机版 readNewsCache(freshOnly=false) 等价：过期缓存也保留在内存里
// 作为联网失败时的兜底，是否新鲜由 GetGalgameNews 判断。
func (s *HomeService) loadNewsCacheLocked() {
	if s.newsCacheLoaded {
		return
	}
	s.newsCacheLoaded = true

	path, err := s.homeNewsCachePath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var payload homeNewsCachePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if len(payload.Items) == 0 {
		return
	}
	s.newsItems = payload.Items
	s.newsFetchedAt = time.UnixMilli(payload.FetchedAt)
}

// writeNewsCacheLocked 落盘缓存（失败只记日志，不影响返回）。
func (s *HomeService) writeNewsCacheLocked(items []vo.HomeNewsItem) {
	path, err := s.homeNewsCachePath()
	if err != nil {
		applog.LogWarningf(s.ctx, "Galgame 资讯：缓存目录不可用：%v", err)
		return
	}
	payload := homeNewsCachePayload{
		FetchedAt: time.Now().UnixMilli(),
		Items:     items,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		applog.LogWarningf(s.ctx, "Galgame 资讯：序列化缓存失败：%v", err)
		return
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		applog.LogWarningf(s.ctx, "Galgame 资讯：写入缓存失败：%v", err)
	}
}

func (s *HomeService) homeNewsCachePath() (string, error) {
	dir, err := apputils.GetCacheSubDir("home")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, homeNewsCacheFileName), nil
}

// isHomeNewsTimeout 识别超时，便于前端区分「加载较慢」与「获取失败」。
func isHomeNewsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
