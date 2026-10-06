package vo

// AccountStatus 是 YukiHub 账号（自建账号服务）的当前状态，供界面渲染。
//
// 令牌本身不回传前端：界面只需要知道「登录了没有、是谁」，
// 具体调用一律由后端完成。
type AccountStatus struct {
	LoggedIn bool   `json:"logged_in"`
	UserID   string `json:"user_id,omitempty"`
	UID      int64  `json:"uid,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`

	KungalBound     bool `json:"kungal_bound"`
	HikarinagiBound bool `json:"hikarinagi_bound"`

	// CloudSyncEnabled：登录后是否自动同步游戏库
	CloudSyncEnabled bool `json:"cloud_sync_enabled"`
	// SharePlaying：关闭时仍上报心跳，但不带「正在玩什么」
	SharePlaying bool `json:"share_playing"`
	// FriendPlayNotify：好友开始玩游戏时是否弹通知（对齐手机版 friend_play_notify）
	FriendPlayNotify bool `json:"friend_play_notify"`

	LastSyncAt   string `json:"last_sync_at,omitempty"`
	LastSyncHash string `json:"last_sync_hash,omitempty"`

	// PresenceActive 表示心跳任务正在跑（登录 + 应用在前台）
	PresenceActive bool `json:"presence_active"`

	ServiceURL string `json:"service_url"`
}

// SelfSyncConfig 是「自持同步（WebDAV）」的配置回显。
//
// 对应手机版 WebDavSettingsDialog 里的三个输入框 + 自动同步开关。
// 密码会回传给前端以支持「保存后仍显示」（手机版同样把密码填回输入框）。
type SelfSyncConfig struct {
	ServerURL string `json:"server_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	AutoSync  bool   `json:"auto_sync"`
	// Configured：URL / 用户名 / 密码三者齐全（对齐手机版 isConfigured）
	Configured bool `json:"configured"`
	// LastSyncAt / LastSyncHash：上次同步时间与快照哈希
	LastSyncAt   string `json:"last_sync_at,omitempty"`
	LastSyncHash string `json:"last_sync_hash,omitempty"`
}

// SelfSyncResult 是一次自持同步的结果。
//
// Action 取值：
//   - uploaded   已上传（本地更新）
//   - downloaded 已下载（云端更新）
//   - merged     已合并（两边都变了，用户选了智能合并）
//   - noop       已是最新
//   - cancelled  用户在冲突对话框里选了取消
//   - conflict   本地与云端都变过，需要用户决定（此时 LocalBytes / RemoteBytes 有值）
type SelfSyncResult struct {
	Action      string `json:"action"`
	Games       int    `json:"games"`
	Sessions    int    `json:"sessions"`
	Imported    int    `json:"imported,omitempty"`
	Skipped     int    `json:"skipped,omitempty"`
	Failed      int    `json:"failed,omitempty"`
	LocalBytes  int    `json:"local_bytes"`
	RemoteBytes int    `json:"remote_bytes"`
	SyncedAt    string `json:"synced_at,omitempty"`
	// UpdatedAt：上传方向才有，云端文件的写入时刻
	UpdatedAt string `json:"updated_at,omitempty"`
}

// AccountSyncResult 是一次云同步的结果。
type AccountSyncResult struct {
	// Action: uploaded / downloaded / merged / noop
	Action   string `json:"action"`
	Games    int    `json:"games"`
	Sessions int    `json:"sessions"`
	// 下载方向才有：实际导入 / 跳过 / 失败的条目数
	Imported  int    `json:"imported"`
	Skipped   int    `json:"skipped"`
	Failed    int    `json:"failed"`
	SyncedAt  string `json:"synced_at,omitempty"`
	Message   string `json:"message,omitempty"`
	RateLimit bool   `json:"rate_limit,omitempty"`
}

// AccountLevel 是等级 / 经验 / 签到状态。
type AccountLevel struct {
	Level             int   `json:"level"`
	Exp               int64 `json:"exp"`
	NextLevelTotalExp int64 `json:"next_level_total_exp"`
	IsMaxLevel        bool  `json:"is_max_level"`
	TodayCheckedIn    bool  `json:"today_checked_in"`
}

// ChatImagePick 是一次聊天图片选择的结果。
//
// Path 为空 = 用户取消；Data 是图片原始字节（上限 500KB，与手机版一致），
// MimeType 是按扩展名推断的 image/jpeg|png|webp。
type ChatImagePick struct {
	Path     string `json:"path"`
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
}

// ChatStickerPack 是一个未萌贴纸包（服务端代理 NextMoe，密钥在服务端）。
type ChatStickerPack struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Cover        string `json:"cover,omitempty"`
	StickerCount int    `json:"sticker_count"`
}

// ChatStickerList 是贴纸包列表；Enabled=false 表示服务端未启用（界面隐藏入口）。
type ChatStickerList struct {
	Enabled bool              `json:"enabled"`
	Packs   []ChatStickerPack `json:"packs"`
}
