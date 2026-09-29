package models

import (
	"time"
	"yukihub/internal/common/enums"
)

// GameMetadataSource identifies a game at one remote metadata provider.
type GameMetadataSource struct {
	GameID     string           `json:"game_id"`
	SourceType enums.SourceType `json:"source_type"`
	SourceID   string           `json:"source_id"`
	// CacheJSON 是该来源的元数据负载，结构沿用 Android 版 VnMetadata，
	// 空串表示尚无缓存。它只是缓存，权威源仍是远程站点。
	CacheJSON string    `json:"cache_json"`
	CachedAt  time.Time `json:"cached_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
