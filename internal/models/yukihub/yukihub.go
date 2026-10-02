package yukihub

// Backup represents a YukiHub backup archive payload.
type Backup struct {
	App       string `json:"app"`
	Schema    int    `json:"schema"`
	CreatedAt int64  `json:"created_at"`
	// Lightweight / Note 是 Android 快照的固定元信息（手机版两个方向都会写）。
	// 云同步快照恒为 lightweight=true；本地全量备份另外带 backup_type。
	Lightweight bool   `json:"lightweight,omitempty"`
	Note        string `json:"note,omitempty"`
	// BackupType 仅本地导出使用：手机版写 "local_full"（云同步快照不带该键）。
	BackupType string `json:"backup_type,omitempty"`
	// Profile 是账号资料段（昵称 / 签名 / 头像 URL）。手机版双向同步它，
	// 桌面端的昵称与头像来自 YukiHub 账号，同样在这里带上。
	Profile       *BackupProfile  `json:"profile,omitempty"`
	Settings      BackupSettings  `json:"settings"`
	Games         []Game          `json:"games"`
	PlaySessions  []PlaySession   `json:"play_sessions"`
	MetadataCache []MetadataCache `json:"metadata_cache"`
}

// BackupProfile 对应手机版快照的 profile 段。
type BackupProfile struct {
	Name string `json:"name,omitempty"`
	// Signature 桌面端没有对应概念，省略该键（手机端 optString(key, 本地值) 会保留原值）。
	Signature string `json:"signature,omitempty"`
	// AvatarURI 必须是 http(s) 地址：手机端会拒收本地 file:// 路径。
	AvatarURI string `json:"avatar_uri"`
}

type BackupSettings struct {
	MetadataSource string `json:"metadata_source"`
}

type Game struct {
	LocalID       int64  `json:"local_id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	// Engine 是 Android 侧的引擎类型，桌面端不存储：导出时留空并省略，
	// 手机端读不到就保留自己那一条的值。
	Engine          string `json:"engine,omitempty"`
	RootUri         string `json:"root_uri"`
	CoverUri        string `json:"cover_uri,omitempty"`
	CoverPersistUri string `json:"cover_persist_uri,omitempty"`
	// CoverSourceType：0 表示「无来源信息」，省略而非写 0。
	CoverSourceType int `json:"cover_source_type,omitempty"`
	// 以下三个字段是 Android 侧的启动方式。桌面端没有对应概念，**必须省略**：
	// 手机端 importGamesJson 用的是 optString(key, 本地值)，字段存在但为空串
	// 会被当作「清空」，直接把对端的模拟器配置抹掉。
	EmulatorPackage    string `json:"emulator_package,omitempty"`
	LaunchTarget       string `json:"launch_target,omitempty"`
	WinlatorLaunchMode string `json:"winlator_launch_mode,omitempty"`
	Description        string `json:"description"`
	Tags               string `json:"tags"`
	// GamehubLocalGameId 是 Android 侧无本地目录条目的身份键，桌面端没有对应概念。
	// 同样必须省略：写成空串会把对端的身份键抹掉，破坏后续匹配。
	GamehubLocalGameId string `json:"gamehub_local_game_id,omitempty"`
	// GaishiLocalGameId 是 gamehub_local_game_id 的旧名。手机版导出时两个键写同值，
	// 导入时优先读 gamehub_*、回退 gaishi_*；桌面端同样两个都写，兼容旧版手机。
	GaishiLocalGameId string `json:"gaishi_local_game_id,omitempty"`
	GamehubLaunchMode string `json:"gamehub_launch_mode,omitempty"`
	PlayStatus        string `json:"play_status"`
	TotalPlayTime     int64  `json:"total_play_time"`
	LastPlayedAt      int64  `json:"last_played_at"`
	PlaytimeResetAt   int64  `json:"playtime_reset_at"` // 清零时间点（Unix 毫秒），0 表示从未清零
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
	Hidden            bool   `json:"hidden"`
	Favorite          bool   `json:"favorite"`
	NSFW              bool   `json:"nsfw"`
}

type PlaySession struct {
	SessionUUID string `json:"session_uuid"`
	GameLocalID int64  `json:"game_local_id"`
	GameRootUri string `json:"game_root_uri"`
	// 桌面端没有 GameHub/gaishi 本地 id 概念，两个键都省略：
	// 写成空串会被手机端 optString(key, 默认值) 命中并当成「确认清空」，
	// 把对端的身份键抹掉（Game 段同理，必须 omitempty）。
	GamehubLocalGameId  string `json:"gamehub_local_game_id,omitempty"`
	GaishiLocalGameId   string `json:"gaishi_local_game_id,omitempty"`
	GameTitle           string `json:"game_title"`
	GameEngine          string `json:"game_engine"`
	GameEmulatorPackage string `json:"game_emulator_package"`
	StartTime           int64  `json:"start_time"`
	EndTime             int64  `json:"end_time"`
	Duration            int64  `json:"duration"` // 毫秒
	LaunchType          string `json:"launch_type"`
	DeviceID            string `json:"device_id"`
	CreatedAt           int64  `json:"created_at"`
	UpdatedAt           int64  `json:"updated_at"`
}

type MetadataCache struct {
	GameLocalID int64  `json:"game_local_id"`
	Source      string `json:"source"`
	SourceID    string `json:"source_id"`
	JSON        string `json:"json"`
	UpdatedAt   int64  `json:"updated_at"`
}

type Metadata struct {
	ID                    string   `json:"id"`
	ChineseTitle          string   `json:"chineseTitle"`
	OriginalTitle         string   `json:"originalTitle"`
	RomanTitle            string   `json:"romanTitle"`
	CoverURL              string   `json:"coverUrl"`
	Description           string   `json:"description"`
	TranslatedDescription string   `json:"translatedDescription"`
	Released              string   `json:"released"`
	Developer             string   `json:"developer"`
	TagsText              string   `json:"tagsText"`
	RatingText            string   `json:"ratingText"`
	CoverSexual           int      `json:"coverSexual"`
	ScreenshotURLs        []string `json:"screenshotUrls"`
}
