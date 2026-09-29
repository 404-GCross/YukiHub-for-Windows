package exporter

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"yukihub/internal/applog"
	"yukihub/internal/common/enums"
	"yukihub/internal/models"
	"yukihub/internal/models/yukihub"
	"yukihub/internal/service/gamehelper"
)

const (
	// yukiHubBackupApp / yukiHubBackupSchema 与 Android 侧快照契约一致。
	yukiHubBackupApp    = "YukiHub"
	yukiHubBackupSchema = 5

	// yukiHubSessionsPerGame 是每个游戏导出的游玩记录上限，与契约的「同步与本地备份均 30 条」一致。
	yukiHubSessionsPerGame = 30

	// Android 侧 launch_type / device_id 的缺省值（见 GameRepository.importPlaySessionsJson）。
	yukiHubLaunchType = "external"
	yukiHubDeviceID   = "desktop"
)

// YukiHubExporter 把桌面端库导出为 Android 版 YukiHub 的 schema 5 快照。
//
// 与 importer 对称：只读 DuckDB，不写库，因此不需要 dbutils.WithDuckDBWriteLock。
type YukiHubExporter struct {
	ctx context.Context
	db  *sql.DB
}

func NewYukiHubExporter(ctx context.Context, db *sql.DB) *YukiHubExporter {
	return &YukiHubExporter{ctx: ctx, db: db}
}

// Build 生成 schema 5 快照。
//
// 刻意不导出：扫描目录配置、自定义背景图/视频、trailer/logo/bg 路径、音乐厅数据、
// 游戏本体与存档、二进制封面图（契约明确排除）。
func (e *YukiHubExporter) Build() (*yukihub.Backup, error) {
	games, err := e.loadGames()
	if err != nil {
		return nil, err
	}
	tags, err := e.loadTags()
	if err != nil {
		return nil, err
	}
	sessions, err := e.loadSessions()
	if err != nil {
		return nil, err
	}
	favorites, err := e.loadFavorites()
	if err != nil {
		return nil, err
	}
	metadataCache, err := e.loadMetadataCache()
	if err != nil {
		return nil, err
	}

	backup := &yukihub.Backup{
		App:       yukiHubBackupApp,
		Schema:    yukiHubBackupSchema,
		CreatedAt: unixMilli(time.Now()),
		Games:     make([]yukihub.Game, 0, len(games)),
		// metadata_cache 取自 game_metadata_sources.cache_json，负载沿用 Android 的
		// VnMetadata 结构，两个方向都能原样往返。
		MetadataCache: metadataCache,
		// settings.metadata_source 是 Android 侧的全局首选来源，桌面端没有对应概念
		// （默认来源是逐游戏的），留空由对端按自身优先级挑选。
	}
	for _, game := range games {
		// 跳过无标题条目：Android 侧会把空标题落成「未命名游戏」，
		// 这类条目在跨端同步时只会制造无法匹配的占位记录。
		if strings.TrimSpace(game.Name) == "" {
			applog.LogWarningf(e.ctx, "跳过无标题游戏（ID=%s），不导出到 YukiHub 快照", game.ID)
			continue
		}
		gameSessions := sessions[game.ID]
		entry, sessionEntries := buildYukiHubGame(game, tags[game.ID], gameSessions, favorites[game.ID])
		backup.Games = append(backup.Games, entry)
		backup.PlaySessions = append(backup.PlaySessions, sessionEntries...)
	}
	return backup, nil
}

// Export 生成快照并以 gzip 写入文件（与 importer 的 gzip 读取对称）。
func (e *YukiHubExporter) Export(path string) error {
	backup, err := e.Build()
	if err != nil {
		return err
	}
	data, err := json.Marshal(backup)
	if err != nil {
		return fmt.Errorf("序列化 YukiHub 快照失败: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建 YukiHub 快照文件失败: %w", err)
	}
	writer := gzip.NewWriter(file)
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		file.Close()
		return fmt.Errorf("写入 YukiHub 快照失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		file.Close()
		return fmt.Errorf("关闭 YukiHub 快照失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("保存 YukiHub 快照失败: %w", err)
	}
	applog.LogInfof(e.ctx, "ExportYukiHub: wrote %d games and %d play sessions", len(backup.Games), len(backup.PlaySessions))
	return nil
}

func (e *YukiHubExporter) loadGames() ([]models.Game, error) {
	query := `SELECT
		g.id,
		COALESCE(g.name, '') as name,
		COALESCE(g.aliases, '[]') as aliases,
		COALESCE(g.cover_url, '') as cover_url,
		COALESCE(g.summary, '') as summary,
		COALESCE(g.path, '') as path,
		COALESCE(g.game_directory, '') as game_directory,
		COALESCE(g.status, 'not_started') as status,
		COALESCE(g.is_nsfw, FALSE) as is_nsfw,
		COALESCE(g.created_at, CURRENT_TIMESTAMP) as created_at,
		COALESCE(g.updated_at, g.created_at, CURRENT_TIMESTAMP) as updated_at,
		COALESCE(g.legacy_local_id, '') as legacy_local_id,
		g.playtime_reset_at,
		COALESCE(g.hidden, FALSE) as hidden
	FROM games g
	ORDER BY g.created_at ASC, g.id ASC`

	rows, err := e.db.QueryContext(e.ctx, query)
	if err != nil {
		applog.LogErrorf(e.ctx, "ExportYukiHub: failed to query games: %v", err)
		return nil, fmt.Errorf("查询游戏列表失败: %w", err)
	}
	defer rows.Close()

	games := make([]models.Game, 0)
	for rows.Next() {
		var game models.Game
		var aliasesJSON string
		var status string
		var playtimeResetAt sql.NullTime
		if err := rows.Scan(
			&game.ID,
			&game.Name,
			&aliasesJSON,
			&game.CoverURL,
			&game.Summary,
			&game.Path,
			&game.GameDirectory,
			&status,
			&game.IsNSFW,
			&game.CreatedAt,
			&game.UpdatedAt,
			&game.LegacyLocalID,
			&playtimeResetAt,
			&game.Hidden,
		); err != nil {
			applog.LogErrorf(e.ctx, "ExportYukiHub: failed to scan game: %v", err)
			return nil, fmt.Errorf("读取游戏记录失败: %w", err)
		}
		game.Aliases, err = gamehelper.DecodeAliases(aliasesJSON)
		if err != nil {
			return nil, fmt.Errorf("解析游戏别名失败: %w", err)
		}
		game.Status = enums.GameStatus(status)
		if playtimeResetAt.Valid {
			resetAt := playtimeResetAt.Time
			game.PlaytimeResetAt = &resetAt
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历游戏列表失败: %w", err)
	}
	return games, nil
}

func (e *YukiHubExporter) loadTags() (map[string][]string, error) {
	rows, err := e.db.QueryContext(e.ctx, "SELECT game_id, name FROM game_tags ORDER BY game_id ASC, name ASC")
	if err != nil {
		applog.LogErrorf(e.ctx, "ExportYukiHub: failed to query game_tags: %v", err)
		return nil, fmt.Errorf("查询游戏标签失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var gameID string
		var name string
		if err := rows.Scan(&gameID, &name); err != nil {
			return nil, fmt.Errorf("读取游戏标签失败: %w", err)
		}
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		result[gameID] = append(result[gameID], name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历游戏标签失败: %w", err)
	}
	return result, nil
}

func (e *YukiHubExporter) loadSessions() (map[string][]models.PlaySession, error) {
	rows, err := e.db.QueryContext(e.ctx, `SELECT
		id, game_id, COALESCE(start_time, CURRENT_TIMESTAMP), end_time, COALESCE(duration, 0), COALESCE(updated_at, start_time, CURRENT_TIMESTAMP)
	FROM play_sessions
	ORDER BY game_id ASC, start_time ASC, id ASC`)
	if err != nil {
		applog.LogErrorf(e.ctx, "ExportYukiHub: failed to query play_sessions: %v", err)
		return nil, fmt.Errorf("查询游玩记录失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]models.PlaySession)
	for rows.Next() {
		var session models.PlaySession
		var endTime sql.NullTime
		if err := rows.Scan(&session.ID, &session.GameID, &session.StartTime, &endTime, &session.Duration, &session.UpdatedAt); err != nil {
			return nil, fmt.Errorf("读取游玩记录失败: %w", err)
		}
		if endTime.Valid {
			session.EndTime = endTime.Time
		}
		result[session.GameID] = append(result[session.GameID], session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历游玩记录失败: %w", err)
	}
	return result, nil
}

// loadFavorites 读取系统收藏分类下的游戏。favorite 在桌面端由「收藏」系统分类承载。
func (e *YukiHubExporter) loadFavorites() (map[string]bool, error) {
	rows, err := e.db.QueryContext(e.ctx, "SELECT game_id FROM game_categories WHERE category_id = ?", gamehelper.SystemFavoritesCategoryID)
	if err != nil {
		applog.LogErrorf(e.ctx, "ExportYukiHub: failed to query favorites: %v", err)
		return nil, fmt.Errorf("查询收藏分类失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var gameID string
		if err := rows.Scan(&gameID); err != nil {
			return nil, fmt.Errorf("读取收藏分类失败: %w", err)
		}
		result[gameID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历收藏分类失败: %w", err)
	}
	return result, nil
}

// loadMetadataCache 读取每个来源的元数据负载，映射为快照的 metadata_cache 元素。
//
// game_local_id 只能由 games.legacy_local_id 还原：Android 的 metadata_cache 以整数
// local_id 关联游戏，桌面端自建条目没有 legacy_local_id，无从关联，只能跳过。
// 无标题游戏与 games 导出保持同一口径（同样跳过），避免产生无法匹配的孤儿缓存。
func (e *YukiHubExporter) loadMetadataCache() ([]yukihub.MetadataCache, error) {
	rows, err := e.db.QueryContext(e.ctx, `SELECT
		COALESCE(g.legacy_local_id, ''),
		s.source_type,
		COALESCE(s.source_id, ''),
		COALESCE(s.cache_json, ''),
		COALESCE(s.updated_at, s.cached_at, g.updated_at, CURRENT_TIMESTAMP)
	FROM game_metadata_sources s
	JOIN games g ON g.id = s.game_id
	WHERE COALESCE(s.cache_json, '') <> ''
	  AND TRIM(COALESCE(g.name, '')) <> ''
	ORDER BY g.created_at ASC, g.id ASC, s.source_type ASC`)
	if err != nil {
		applog.LogErrorf(e.ctx, "ExportYukiHub: failed to query metadata cache: %v", err)
		return nil, fmt.Errorf("查询元数据缓存失败: %w", err)
	}
	defer rows.Close()

	entries := make([]yukihub.MetadataCache, 0)
	for rows.Next() {
		var legacyLocalID string
		var sourceType string
		var sourceID string
		var payload string
		var updatedAt time.Time
		if err := rows.Scan(&legacyLocalID, &sourceType, &sourceID, &payload, &updatedAt); err != nil {
			return nil, fmt.Errorf("读取元数据缓存失败: %w", err)
		}
		localID := parseYukiHubLocalID(legacyLocalID)
		if localID <= 0 {
			continue
		}
		entries = append(entries, yukihub.MetadataCache{
			GameLocalID: localID,
			Source:      string(gamehelper.NormalizeMetadataSourceType(enums.SourceType(sourceType))),
			SourceID:    strings.TrimSpace(sourceID),
			JSON:        payload,
			UpdatedAt:   unixMilli(updatedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历元数据缓存失败: %w", err)
	}
	return entries, nil
}

// buildYukiHubGame 把一个桌面端游戏转换为快照的 games 元素，并生成它的游玩记录。
func buildYukiHubGame(game models.Game, tags []string, sessions []models.PlaySession, favorite bool) (yukihub.Game, []yukihub.PlaySession) {
	resetAt := time.Time{}
	if game.PlaytimeResetAt != nil {
		resetAt = *game.PlaytimeResetAt
	}
	// 与 Android 侧 exportPlaySessionsJson 对称：只保留清零之后的会话，否则清零历史会在 Android 侧复活。
	kept := sessionsAfterReset(sessions, resetAt)

	var totalPlayTime int64
	var lastPlayedAt int64
	for _, session := range kept {
		totalPlayTime += int64(session.Duration) * 1000 // 桌面端单位是秒，Android 侧是毫秒
		if playedAt := sessionTimestamp(session); playedAt > lastPlayedAt {
			lastPlayedAt = playedAt
		}
	}

	entry := yukihub.Game{
		LocalID:       parseYukiHubLocalID(game.LegacyLocalID),
		Title:         game.Name,
		OriginalTitle: firstYukiHubAlias(game.Aliases),
		Engine:        "", // Android 专用（引擎类型），桌面端不存储
		// RootUri 恒为空串，不导出 Windows 绝对路径。原因：
		// 1) 这些路径在 Android 上不可达，写过去只会让对端存一堆无意义的 D:\... ；
		// 2) 与 importer 方向对称——桌面端导入 Android 备份时同样不把对端的
		//    root_uri 写进桌面端 path/game_directory；
		// 3) 契约已明确「两侧规范化后的路径字符串不等价，跨端匹配以标题为主」。
		// 空 root_uri 时 Android 会走 findByTitleForEmptyRoot 按标题匹配。
		// TODO(round-trip)：Android 源生的条目（其本地 root_uri 非空）经桌面端
		// 回导时仍可能在对端产生重复。要彻底解决需新增 legacy_root_uri 列保存
		// 对端原始路径并在导出时回填，见 ROADMAP 阶段 2。
		RootUri:         "",
		CoverUri:        networkCoverURI(game.CoverURL),
		CoverPersistUri: "",
		CoverSourceType: 0,
		Description:     game.Summary,
		Tags:            strings.Join(tags, ","),
		PlayStatus:      mapGameStatusToYukiHub(game.Status),
		TotalPlayTime:   totalPlayTime,
		LastPlayedAt:    lastPlayedAt,
		PlaytimeResetAt: unixMilli(resetAt),
		CreatedAt:       unixMilli(game.CreatedAt),
		UpdatedAt:       unixMilli(game.UpdatedAt),
		Hidden:          game.Hidden,
		Favorite:        favorite,
		NSFW:            game.IsNSFW,
	}
	return entry, buildYukiHubSessions(entry, game, kept)
}

func buildYukiHubSessions(entry yukihub.Game, game models.Game, sessions []models.PlaySession) []yukihub.PlaySession {
	// 只导出最新的 yukiHubSessionsPerGame 条（契约规定按时间取尾部），sessions 已按开始时间升序。
	exported := sessions
	if len(exported) > yukiHubSessionsPerGame {
		exported = exported[len(exported)-yukiHubSessionsPerGame:]
	}

	result := make([]yukihub.PlaySession, 0, len(exported))
	for _, session := range exported {
		result = append(result, yukihub.PlaySession{
			SessionUUID: session.ID,
			GameLocalID: entry.LocalID,
			GameRootUri: entry.RootUri,
			GameTitle:   game.Name,
			StartTime:   unixMilli(session.StartTime),
			EndTime:     unixMilli(session.EndTime),
			Duration:    int64(session.Duration) * 1000, // 秒 → 毫秒
			LaunchType:  yukiHubLaunchType,
			DeviceID:    yukiHubDeviceID,
			CreatedAt:   unixMilli(session.StartTime),
			UpdatedAt:   unixMilli(session.UpdatedAt),
		})
	}
	return result
}

// sessionsAfterReset 复刻 Android 侧 exportPlaySessionsJson 的
// COALESCE(ps.end_time, ps.start_time, 0) >= IFNULL(g.playtime_reset_at, 0) 条件。
func sessionsAfterReset(sessions []models.PlaySession, resetAt time.Time) []models.PlaySession {
	resetMillis := unixMilli(resetAt)
	result := make([]models.PlaySession, 0, len(sessions))
	for _, session := range sessions {
		if sessionTimestamp(session) < resetMillis {
			continue
		}
		result = append(result, session)
	}
	// 保证「取最新若干条」与「会话顺序」都稳定，避免同秒会话在不同机器上落到不同结果。
	sort.SliceStable(result, func(i, j int) bool {
		left := sessionTimestamp(result[i])
		right := sessionTimestamp(result[j])
		if left != right {
			return left < right
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// sessionTimestamp 取会话的时间点：优先 end_time，缺失时回退 start_time。
func sessionTimestamp(session models.PlaySession) int64 {
	if !session.EndTime.IsZero() {
		return unixMilli(session.EndTime)
	}
	return unixMilli(session.StartTime)
}

// mapGameStatusToYukiHub 把桌面端 6 态映射为 Android 侧 5 态。
// Android 没有「想玩」，want_to_play 按契约降级为 unplayed；未知值同样回落 unplayed。
func mapGameStatusToYukiHub(status enums.GameStatus) string {
	switch status {
	case enums.StatusPlaying:
		return "playing"
	case enums.StatusCompleted:
		return "completed"
	case enums.StatusOnHold:
		return "onhold"
	case enums.StatusDropped:
		return "dropped"
	default:
		return "unplayed"
	}
}

// networkCoverURI 只导出网络封面，本地封面文件跨设备无效（契约明确不迁移）。
func networkCoverURI(coverURL string) string {
	coverURL = strings.TrimSpace(coverURL)
	if !strings.HasPrefix(coverURL, "http://") && !strings.HasPrefix(coverURL, "https://") {
		return ""
	}
	return coverURL
}

func firstYukiHubAlias(aliases []string) string {
	for _, alias := range aliases {
		if alias = strings.TrimSpace(alias); alias != "" {
			return alias
		}
	}
	return ""
}

// parseYukiHubLocalID 把保留的 Android 侧整数 ID 还原为 local_id，缺失或非法时为 0。
// Android 侧把 0 当作「无身份」，不会用它做匹配。
func parseYukiHubLocalID(value string) int64 {
	localID, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || localID <= 0 {
		return 0
	}
	return localID
}

func unixMilli(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}
