// Package yukihubaccount 是 YukiHub 自建账号服务（https://yukihub.zh.kg/api）的客户端。
//
// 接口契约与 YukiHub Android 版保持一致（参见 Android 侧的 AuthActivity /
// MainActivity / SyncManager / SocialApiClient），桌面端只是多加了一个调用方。
// 几个必须照做的细节：
//
//   - 登录 / 注册 / 发验证码走 **GET + query string**（后端既有设计，密码也在 URL 里）；
//     重置密码、刷新令牌、改昵称等走 POST + JSON。
//   - 认证统一是 `Authorization: Bearer <access_token>`。
//   - 请求要带正常的 User-Agent 与 `Referer: https://yukihub.zh.kg/`：
//     后端前置了 Cloudflare，UA 异常会被按浏览器指纹拦掉（Error 1010）。
//   - 云同步接口收发的是 **gzip 原始字节**，不是 JSON；`/sync/download` 返回 404
//     表示云端还没有数据（不是错误）。
//   - 响应字段名有不一致的历史（accessToken / access_token / token…），
//     所以解析一律走「别名列表」而不是直接反序列化到结构体。
package yukihubaccount

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"yukihub/internal/applog"
	"yukihub/internal/version"
)

const (
	// DefaultBaseURL 是 YukiHub 账号服务的默认地址。
	DefaultBaseURL = "https://yukihub.zh.kg/api"
	// defaultSiteURL 用于 Referer 头（后端按它校验来源）。
	defaultSiteURL = "https://yukihub.zh.kg/"

	// 单个响应体的读取上限，纯属防呆。
	maxResponseBytes = 32 << 20
	// 上传同步快照的接口可能比较慢，单独给一个更长的超时。
	syncRequestTimeout = 90 * time.Second
	defaultTimeout     = 25 * time.Second
)

// ErrNotLoggedIn 表示本地没有可用令牌。
var ErrNotLoggedIn = errors.New("未登录 YukiHub 账号")

// ErrUnauthorized 表示访问令牌失效（HTTP 401）。调用方据此刷新令牌并重试一次。
var ErrUnauthorized = errors.New("登录状态已失效")

// ErrCloudSnapshotMissing 表示云端还没有同步数据（后端返回 404）。
var ErrCloudSnapshotMissing = errors.New("云端还没有同步数据")

// Session 是一次成功认证后的结果。
type Session struct {
	AccessToken  string
	RefreshToken string
	User         User
}

// User 是账号的基本资料。
type User struct {
	ID              string
	UID             int64
	Nickname        string
	Email           string
	Avatar          string
	KungalBound     bool
	HikarinagiBound bool
}

// PublicProfile 是别人的公开资料（查看好友时用）。
type PublicProfile struct {
	Nickname      string
	UID           int64
	Signature     string
	AvatarURL     string
	Status        string
	Activity      string
	TotalGames    int
	TotalPlayTime int64
}

// LevelInfo 是等级 / 经验 / 签到状态。
type LevelInfo struct {
	Level             int
	Exp               int64
	NextLevelTotalExp int64
	IsMaxLevel        bool
	TodayCheckedIn    bool
}

// Client 是账号服务的 HTTP 客户端。
type Client struct {
	baseURL string
	siteURL string
	http    *http.Client
	now     func() time.Time
}

// NewClient 创建客户端；baseURL 为空时用 DefaultBaseURL。
func NewClient(baseURL string) *Client {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		trimmed = DefaultBaseURL
	}
	return &Client{
		baseURL: trimmed,
		siteURL: defaultSiteURL,
		http:    &http.Client{Timeout: defaultTimeout},
		now:     time.Now,
	}
}

// SetHTTPClient 允许注入自定义客户端（测试与代理场景）。
func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.http = client
	}
}

// ==================== 认证 ====================

// SendCode 发送邮箱验证码。purpose 为 "register" 时走注册接口，其余走找回密码接口。
func (c *Client) SendCode(ctx context.Context, email string, purpose string) error {
	path := "/auth/send_code"
	if purpose == CodePurposeReset {
		path = "/auth/send_reset_code"
	}
	query := url.Values{"email": {strings.TrimSpace(email)}}
	return c.doEmpty(ctx, http.MethodGet, c.baseURL+path+"?"+query.Encode(), "", nil)
}

// 验证码用途。
const (
	CodePurposeRegister = "register"
	CodePurposeReset    = "reset"
)

// Register 用邮箱 + 验证码注册新账号。
func (c *Client) Register(ctx context.Context, email, password, nickname, code string) (Session, error) {
	query := url.Values{
		"email":    {strings.TrimSpace(email)},
		"password": {password},
		"nickname": {strings.TrimSpace(nickname)},
		"code":     {strings.TrimSpace(code)},
	}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/auth/register?"+query.Encode(), "", nil)
	if err != nil {
		return Session{}, err
	}
	return parseSession(body)
}

// Login 用邮箱 + 密码登录。
func (c *Client) Login(ctx context.Context, email, password string) (Session, error) {
	query := url.Values{
		"email":    {strings.TrimSpace(email)},
		"password": {password},
	}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/auth/login?"+query.Encode(), "", nil)
	if err != nil {
		return Session{}, err
	}
	return parseSession(body)
}

// Refresh 用 refresh token 换新的 access token。
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	payload := map[string]string{"refreshToken": strings.TrimSpace(refreshToken)}
	body, err := c.doJSON(ctx, http.MethodPost, c.baseURL+"/auth/refresh", "", payload)
	if err != nil {
		return Session{}, err
	}
	return parseSession(body)
}

// ResetPassword 用邮箱验证码重置密码。
func (c *Client) ResetPassword(ctx context.Context, email, code, password string) error {
	payload := map[string]string{
		"email":    strings.TrimSpace(email),
		"code":     strings.TrimSpace(code),
		"password": password,
	}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/auth/reset_password", "", payload)
}

// Health 探测服务是否可用。
func (c *Client) Health(ctx context.Context) error {
	return c.doEmpty(ctx, http.MethodGet, c.baseURL+"/health", "", nil)
}

// ==================== 资料 ====================

// GetPublicProfile 查看某个用户的公开资料。
func (c *Client) GetPublicProfile(ctx context.Context, token string, uid int64) (PublicProfile, error) {
	query := url.Values{"uid": {strconv.FormatInt(uid, 10)}}
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/user/profile?"+query.Encode(), token, nil)
	if err != nil {
		return PublicProfile{}, err
	}
	profile := PublicProfile{
		Nickname:  pickString(body, "nickname", "name"),
		UID:       pickInt64(body, "uid", "id"),
		Signature: pickString(body, "signature"),
		AvatarURL: pickString(body, "avatarUrl", "avatar_url", "avatar"),
		Status:    pickString(body, "status"),
		Activity:  pickString(body, "activity"),
	}
	if value, ok := body["totalGames"]; ok {
		profile.TotalGames = int(toInt64(value))
	}
	if value, ok := body["totalPlayTime"]; ok {
		profile.TotalPlayTime = toInt64(value)
	}
	return profile, nil
}

// UpdateNickname 修改云端昵称。
func (c *Client) UpdateNickname(ctx context.Context, token, nickname string) error {
	payload := map[string]string{"nickname": strings.TrimSpace(nickname)}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/user/update_nickname", token, payload)
}

// GetLevel 查询等级 / 经验 / 今日签到状态。
func (c *Client) GetLevel(ctx context.Context, token string) (LevelInfo, error) {
	body, err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/user/level", token, nil)
	if err != nil {
		return LevelInfo{}, err
	}
	return LevelInfo{
		Level:             int(pickInt64(body, "level")),
		Exp:               pickInt64(body, "exp"),
		NextLevelTotalExp: pickInt64(body, "nextLevelTotalExp"),
		IsMaxLevel:        pickBool(body, "isMaxLevel"),
		TodayCheckedIn:    pickBool(body, "todayCheckedIn"),
	}, nil
}

// UploadAvatar 上传头像原始 JPEG 字节，返回服务器上的头像地址。
func (c *Client) UploadAvatar(ctx context.Context, token string, jpegData []byte) (string, error) {
	body, err := c.doRaw(ctx, http.MethodPost, c.baseURL+"/upload_avatar", token, "image/jpeg", jpegData)
	if err != nil {
		return "", err
	}
	avatar := pickString(body, "avatarUrl", "avatar_url", "avatar")
	if avatar == "" {
		return "", errors.New("头像上传没有返回地址")
	}
	return avatar, nil
}

// ==================== 云同步 ====================

// UploadSnapshot 上传 gzip 压缩后的快照字节。
func (c *Client) UploadSnapshot(ctx context.Context, token string, gzipped []byte) error {
	body, err := c.doRawWithTimeout(ctx, http.MethodPost, c.baseURL+"/sync/upload", token,
		"application/octet-stream", gzipped, syncRequestTimeout)
	if err != nil {
		return err
	}
	// 后端约定：成功必须回 success:true，否则读 error 文案。
	if !pickBool(body, "success", "ok") {
		if message := pickString(body, "error", "message"); message != "" {
			return errors.New(message)
		}
		return errors.New("云端拒绝了这次上传")
	}
	return nil
}

// DownloadSnapshot 下载云端快照（已解 gzip）。云端无数据时返回 ErrCloudSnapshotMissing。
func (c *Client) DownloadSnapshot(ctx context.Context, token string) ([]byte, error) {
	request, err := c.newRequest(ctx, http.MethodGet, c.baseURL+"/sync/download", token, "", nil)
	if err != nil {
		return nil, err
	}
	client := *c.http
	client.Timeout = syncRequestTimeout

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载云端快照失败: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound {
		return nil, ErrCloudSnapshotMissing
	}
	raw, err := readLimitedBody(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载云端快照失败（HTTP %d）：%s", response.StatusCode, summarizeBody(raw))
	}
	return gunzipIfNeeded(raw)
}

// ==================== 在线状态 ====================

// Heartbeat 上报在线状态。activity 为空串表示清除「正在玩」。
func (c *Client) Heartbeat(ctx context.Context, token, status, activity string) error {
	payload := map[string]string{"status": status, "activity": activity}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/presence/heartbeat", token, payload)
}

// MarkOffline 尽力上报一次下线（失败也不影响本地登出）。
func (c *Client) MarkOffline(ctx context.Context, token string) error {
	payload := map[string]string{"status": "offline", "activity": ""}
	return c.doEmpty(ctx, http.MethodPost, c.baseURL+"/presence/offline", token, payload)
}

// ==================== 内部：请求与解析 ====================

func (c *Client) newRequest(ctx context.Context, method, rawURL, token, contentType string, payload any) (*http.Request, error) {
	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("序列化请求失败: %w", err)
		}
		reader = bytes.NewReader(encoded)
		if contentType == "" {
			contentType = "application/json"
		}
	}

	request, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	// Cloudflare 会按 UA 指纹拦截，必须带正常 UA；Referer 是后端自己的来源校验。
	request.Header.Set("User-Agent", version.UserAgent())
	request.Header.Set("Referer", c.siteURL)
	request.Header.Set("Accept", "application/json, */*")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request, nil
}

func (c *Client) doJSON(ctx context.Context, method, rawURL, token string, payload any) (map[string]any, error) {
	return c.doRaw(ctx, method, rawURL, token, "", payload)
}

// doRaw 发请求并解析 JSON 响应；payload 非 []byte 时按 JSON 序列化。
func (c *Client) doRaw(ctx context.Context, method, rawURL, token, contentType string, payload any) (map[string]any, error) {
	return c.doRawWithTimeout(ctx, method, rawURL, token, contentType, payload, defaultTimeout)
}

func (c *Client) doRawWithTimeout(ctx context.Context, method, rawURL, token, contentType string, payload any, timeout time.Duration) (map[string]any, error) {
	var request *http.Request
	var err error
	if rawBytes, ok := payload.([]byte); ok {
		request, err = c.newRequest(ctx, method, rawURL, token, contentType, nil)
		if err == nil {
			request.Body = io.NopCloser(bytes.NewReader(rawBytes))
			request.ContentLength = int64(len(rawBytes))
		}
	} else {
		request, err = c.newRequest(ctx, method, rawURL, token, contentType, payload)
	}
	if err != nil {
		return nil, err
	}

	client := *c.http
	if timeout > 0 {
		client.Timeout = timeout
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求账号服务失败: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	raw, err := readLimitedBody(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, buildHTTPError(response.StatusCode, raw)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		// 少数端点会返回非 JSON（例如 /health 的纯文本），此时不当作错误。
		return map[string]any{}, nil
	}
	return decoded, nil
}

// doEmpty 只关心成功与否。
func (c *Client) doEmpty(ctx context.Context, method, rawURL, token string, payload any) error {
	_, err := c.doJSON(ctx, method, rawURL, token, payload)
	return err
}

// buildHTTPError 把后端的错误文案翻出来给界面用。
func buildHTTPError(statusCode int, raw []byte) error {
	message := ""
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err == nil {
		message = pickString(decoded, "error", "message", "msg")
	}
	if message == "" {
		message = summarizeBody(raw)
	}
	switch statusCode {
	case http.StatusUnauthorized:
		if message == "" {
			message = "登录状态已失效"
		}
		// 包一层哨兵错误，调用方据此刷新令牌后重试一次。
		return fmt.Errorf("%w: %s", ErrUnauthorized, message)
	case http.StatusForbidden:
		if message == "" {
			message = "账号被限制使用"
		}
		return fmt.Errorf("%s", message)
	default:
		if message == "" {
			return fmt.Errorf("账号服务返回 HTTP %d", statusCode)
		}
		return fmt.Errorf("%s", message)
	}
}

func summarizeBody(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	runes := []rune(text)
	if len(runes) > 200 {
		return string(runes[:200]) + "…"
	}
	return text
}

func readLimitedBody(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, errors.New("响应体过大，已中止")
	}
	return data, nil
}

// gunzipIfNeeded 解 gzip；如果不是 gzip（老格式是纯 JSON）就原样返回。
func gunzipIfNeeded(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("解压云端快照失败: %w", err)
	}
	defer func() { _ = reader.Close() }()

	plain, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("解压云端快照失败: %w", err)
	}
	if len(plain) > maxResponseBytes {
		return nil, errors.New("云端快照过大，已中止")
	}
	return plain, nil
}

// GzipBytes 按手机版的同一口径压缩（gzip 默认级别）。
func GzipBytes(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("压缩快照失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("压缩快照失败: %w", err)
	}
	return buffer.Bytes(), nil
}

// parseSession 解析登录 / 注册 / 刷新的响应。
//
// 后端历史上换过字段名（accessToken / access_token / token），
// 用户对象也可能在 user 或 data.user 下，所以逐个候选取值。
func parseSession(body map[string]any) (Session, error) {
	session := Session{
		AccessToken:  pickString(body, "accessToken", "access_token", "token"),
		RefreshToken: pickString(body, "refreshToken", "refresh_token"),
	}
	if session.AccessToken == "" {
		// 有些响应把会话挂在 data 下
		if data, ok := body["data"].(map[string]any); ok {
			session.AccessToken = pickString(data, "accessToken", "access_token", "token")
			if session.RefreshToken == "" {
				session.RefreshToken = pickString(data, "refreshToken", "refresh_token")
			}
		}
	}
	if session.AccessToken == "" {
		if message := pickString(body, "error", "message", "msg"); message != "" {
			return Session{}, errors.New(message)
		}
		return Session{}, errors.New("账号服务没有返回访问令牌")
	}
	session.User = parseUser(body)
	return session, nil
}

func parseUser(body map[string]any) User {
	// 用户对象可能在 user / data.user / data，找不到就退回顶层。
	source := body
	if nested, ok := body["user"].(map[string]any); ok {
		source = nested
	} else if data, ok := body["data"].(map[string]any); ok {
		if nested, ok := data["user"].(map[string]any); ok {
			source = nested
		} else {
			source = data
		}
	}

	user := User{
		ID:              pickString(source, "id", "userId", "user_id"),
		UID:             pickInt64(source, "uid"),
		Nickname:        pickString(source, "nickname", "name", "username"),
		Email:           pickString(source, "email"),
		Avatar:          pickString(source, "avatarUrl", "avatar_url", "avatar"),
		KungalBound:     pickBool(source, "kungalBound", "kungal_bound"),
		HikarinagiBound: pickBool(source, "hikarinagiBound", "hikarinagi_bound"),
	}
	if user.ID == "" {
		user.ID = pickString(body, "id", "userId", "user_id")
	}
	if user.Nickname == "" {
		user.Nickname = pickString(body, "nickname", "name", "username")
	}
	if user.Email == "" {
		user.Email = pickString(body, "email")
	}
	applog.LogInfof(context.Background(), "YukiHub 账号：解析到用户 uid=%d nickname=%q", user.UID, user.Nickname)
	return user
}

func pickString(source map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := source[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case float64:
			return strconv.FormatInt(int64(typed), 10)
		case json.Number:
			return typed.String()
		}
	}
	return ""
}

func pickBool(source map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := source[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			lowered := strings.ToLower(strings.TrimSpace(typed))
			return lowered == "true" || lowered == "1"
		case float64:
			return typed != 0
		}
	}
	return false
}

func pickInt64(source map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := source[key]
		if !ok || value == nil {
			continue
		}
		return toInt64(value)
	}
	return 0
}

func toInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return parsed
		}
		// uid 也可能是 "42.0" 这种
		if floatValue, floatErr := strconv.ParseFloat(strings.TrimSpace(typed), 64); floatErr == nil {
			return int64(floatValue)
		}
	}
	return 0
}
