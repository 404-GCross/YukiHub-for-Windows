package vo

// HomeNewsItem 首页「Galgame 资讯」的一条。
//
// 字段与手机版 HomeActivity.NewsItem 一一对应，取值直接来自 NextMoe
// /v2/news 的响应（无信封集合 items[]）。
type HomeNewsItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	// BannerURL 是题图（手机版当前源覆盖率 100%，仍做空值兜底）。
	BannerURL string `json:"banner_url"`
	// SourceURL 是原文地址（详情弹窗的「阅读原文」）。
	SourceURL string `json:"source_url"`
	// Attribution 是来源署名：NextMoe 要求引用资讯必须标注来源。
	Attribution string `json:"attribution"`
	PublishedAt string `json:"published_at"`
}

// HomeNewsResult 是首页资讯的返回。
//
// Items 为空表示「没有可用内容」，前端据此显示提示文案；Stale 表示本次
// 联网失败、用的是过期缓存（与手机版一致：静默展示旧内容，不打扰用户）。
// TimedOut 用于区分头部文案（手机版也有「加载较慢」与「获取失败」两种）。
type HomeNewsResult struct {
	Items    []HomeNewsItem `json:"items"`
	Stale    bool           `json:"stale"`
	TimedOut bool           `json:"timed_out"`
}
