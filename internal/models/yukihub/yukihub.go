package yukihub

// Backup represents a YukiHub backup archive payload.
type Backup struct {
	App           string          `json:"app"`
	Schema        int             `json:"schema"`
	CreatedAt     int64           `json:"created_at"`
	Settings      BackupSettings  `json:"settings"`
	Games         []Game          `json:"games"`
	PlaySessions  []PlaySession   `json:"play_sessions"`
	MetadataCache []MetadataCache `json:"metadata_cache"`
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
	GamehubLaunchMode  string `json:"gamehub_launch_mode,omitempty"`
	PlayStatus         string `json:"play_status"`
	TotalPlayTime      int64  `json:"total_play_time"`
	LastPlayedAt       int64  `json:"last_played_at"`
	PlaytimeResetAt    int64  `json:"playtime_reset_at"` // 清零时间点（Unix 毫秒），0 表示从未清零
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
	Hidden             bool   `json:"hidden"`
	Favorite           bool   `json:"favorite"`
	NSFW               bool   `json:"nsfw"`
}

type PlaySession struct {
	SessionUUID         string `json:"session_uuid"`
	GameLocalID         int64  `json:"game_local_id"`
	GameRootUri         string `json:"game_root_uri"`
	GamehubLocalGameId  string `json:"gamehub_local_game_id"`
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
