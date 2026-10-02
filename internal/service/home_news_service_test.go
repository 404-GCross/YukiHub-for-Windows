package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"yukihub/internal/common/vo"
)

// 真实响应片段（取自 api.nextmoe.dev/v2/news?limit=6&nsfw=true 的实测结果）。
const nextMoeNewsSample = `{
  "$schema": "https://api.nextmoe.dev/schemas/news_list.json",
  "object": "list",
  "items": [
    {
      "object": "news_item",
      "id": "4706",
      "title": "扯断提线！在那之后迎接人偶会是无尽的坠落吗？",
      "summary": "这个小小的幼女有着水蓝色，天青色的瞳孔。",
      "source": {
        "object": "news_source",
        "name": "ymgal",
        "display_name": "月幕 Galgame",
        "attribution": "本条情报转载自月幕 Galgame,点击标题可跳转至月幕原文"
      },
      "source_url": "https://www.ymgal.games/co/article/890020904338194432",
      "lane": "column",
      "banner": {
        "url": "https://image.kungal.iloveren.link/42/2c/abc.webp",
        "hash": "abc"
      },
      "published_at": "2026-10-01T08:25:54Z",
      "has_body": false
    },
    {
      "id": "4707",
      "title": "   ",
      "summary": "无标题条目应被跳过",
      "source_url": "https://example.com/skip",
      "published_at": "2026-10-01T09:00:00Z"
    },
    {
      "id": "4708",
      "title": "第二条有效资讯",
      "summary": "",
      "banner": { "url": "" },
      "published_at": "2026-10-01T10:00:00Z"
    }
  ],
  "next_cursor": null
}`

// TestParseNextMoeNewsMatchesAndroid 钉住解析契约。
//
// 手机版 parseNewsJson 的规则：字段名 title/summary/banner.url/source_url/
// source.attribution/published_at；**无标题条目跳过**（没法点）；
// 条数上限 6。这些一旦跑偏，首页就是空白或错位。
func TestParseNextMoeNewsMatchesAndroid(t *testing.T) {
	t.Parallel()

	items, err := parseNextMoeNews([]byte(nextMoeNewsSample))
	if err != nil {
		t.Fatalf("parseNextMoeNews: %v", err)
	}
	// 第三条的有效性 + 第二条被跳过
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2（无标题条目必须跳过）", len(items))
	}

	first := items[0]
	if first.ID != "4706" {
		t.Errorf("id = %q, want 4706", first.ID)
	}
	if first.Title != "扯断提线！在那之后迎接人偶会是无尽的坠落吗？" {
		t.Errorf("title = %q", first.Title)
	}
	if first.Summary != "这个小小的幼女有着水蓝色，天青色的瞳孔。" {
		t.Errorf("summary = %q", first.Summary)
	}
	if first.BannerURL != "https://image.kungal.iloveren.link/42/2c/abc.webp" {
		t.Errorf("banner_url = %q（应取 banner.url）", first.BannerURL)
	}
	if first.SourceURL != "https://www.ymgal.games/co/article/890020904338194432" {
		t.Errorf("source_url = %q", first.SourceURL)
	}
	if !strings.Contains(first.Attribution, "月幕 Galgame") {
		t.Errorf("attribution = %q（应取 source.attribution，详情页必须展示署名）",
			first.Attribution)
	}
	if first.PublishedAt != "2026-10-01T08:25:54Z" {
		t.Errorf("published_at = %q", first.PublishedAt)
	}

	if items[1].Title != "第二条有效资讯" {
		t.Errorf("第二条 = %q", items[1].Title)
	}
}

// TestParseNextMoeNewsCapsAtSix 条数上限与手机版 NEWS_ITEM_COUNT 一致。
func TestParseNextMoeNewsCapsAtSix(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	builder.WriteString(`{"items":[`)
	for i := 0; i < 10; i++ {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(`{"id":"1","title":"条目标题"}`)
	}
	builder.WriteString(`]}`)

	items, err := parseNextMoeNews([]byte(builder.String()))
	if err != nil {
		t.Fatalf("parseNextMoeNews: %v", err)
	}
	if len(items) != nextMoeNewsItemCount {
		t.Errorf("items = %d, want %d", len(items), nextMoeNewsItemCount)
	}
	if nextMoeNewsItemCount != 6 {
		t.Errorf("nextMoeNewsItemCount = %d, want 6（与手机版保持一致）", nextMoeNewsItemCount)
	}
}

// TestParseNextMoeNewsRejectsGarbage 坏响应必须报错，而不是静默给出空列表
// （静默空列表会让前端一直显示「获取失败」而日志里什么都看不到）。
func TestParseNextMoeNewsRejectsGarbage(t *testing.T) {
	t.Parallel()

	if _, err := parseNextMoeNews([]byte("<html>502 Bad Gateway</html>")); err == nil {
		t.Error("非 JSON 响应应当报错")
	}
}

// TestIsHomeNewsTimeout 超时识别：前端据此区分「加载较慢」与「获取失败」。
func TestIsHomeNewsTimeout(t *testing.T) {
	t.Parallel()

	if !isHomeNewsTimeout(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded 应识别为超时")
	}
	if !isHomeNewsTimeout(timeoutError{}) {
		t.Error("net.Error 超时应识别为超时")
	}
	if isHomeNewsTimeout(errors.New("connection refused")) {
		t.Error("普通错误不应识别为超时")
	}
	if isHomeNewsTimeout(nil) {
		t.Error("nil 不应识别为超时")
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ net.Error = timeoutError{}

// TestHomeNewsCacheRoundTrip 缓存结构能原样往返（落盘格式与手机版同构：
// fetched_at 毫秒 + items 数组）。
func TestHomeNewsCacheRoundTrip(t *testing.T) {
	t.Parallel()

	payload := homeNewsCachePayload{
		FetchedAt: time.Now().UnixMilli(),
		Items: []vo.HomeNewsItem{
			{ID: "1", Title: "标题", BannerURL: "https://a/b.webp"},
		},
	}
	raw, err := marshalHomeNewsCache(payload)
	if err != nil {
		t.Fatalf("marshalHomeNewsCache: %v", err)
	}
	parsed, ok := unmarshalHomeNewsCache(raw)
	if !ok {
		t.Fatal("unmarshalHomeNewsCache 失败")
	}
	if parsed.FetchedAt != payload.FetchedAt {
		t.Errorf("fetched_at = %d, want %d", parsed.FetchedAt, payload.FetchedAt)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].Title != "标题" {
		t.Errorf("items 往返失败: %#v", parsed.Items)
	}
}
