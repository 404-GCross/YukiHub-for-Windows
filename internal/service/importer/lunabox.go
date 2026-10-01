package importer

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"yukihub/internal/applog"
	"yukihub/internal/common/enums"
	"yukihub/internal/common/vo"
	"yukihub/internal/models"
)

// LunaBoxImporter 从上游 LunaBox 的数据库导入游戏与游玩记录。
//
// LunaBox 是本仓库的硬分叉：13 张表同名、games 表字段也基本一致，
// 所以不需要格式转换，按列名直接映射即可。但有两处**必须显式处理**，
// 否则不会报错、只会「静默归错」：
//
//   - games.status：LunaBox 是 not_started / want_to_play / on_hold，
//     本仓库按手机版规范合并为 unplayed / onhold（没有「想玩」这一态）
//   - games 少了 5 列（legacy_local_id、source_device_id、playtime_reset_at、
//     hidden、trailer_path），这些按零值处理
//
// 读取刻意按**列名**取值、而不是按固定列顺序 Scan：
// 既不怕 LunaBox 少那 5 列，也能直接读本仓库自己的库文件（互为备份）。
type LunaBoxImporter struct {
	deps Dependencies
}

// NewLunaBoxImporter 创建 LunaBox 导入器。
func NewLunaBoxImporter(deps Dependencies) *LunaBoxImporter {
	return &LunaBoxImporter{deps: deps}
}

// Preview 预览 LunaBox 数据库中的游戏。
func (l *LunaBoxImporter) Preview(dbPath string) ([]PreviewGame, error) {
	games, _, err := loadLunaBoxData(dbPath)
	if err != nil {
		applog.LogErrorf(l.deps.Ctx, "PreviewLunaBoxImport: 读取数据库失败: %v", err)
		return nil, err
	}

	existingGames, _, _, err := l.deps.existingGames("PreviewLunaBoxImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(games))
	for _, game := range games {
		if game.Name == "" {
			continue
		}
		conflict := previewConflict(existingIndex, game.Name, game.Path, string(game.SourceType), game.SourceID)
		previews = append(previews, PreviewGame{
			Name:         game.Name,
			Developer:    game.Company,
			SourceType:   string(game.SourceType),
			SourceID:     game.SourceID,
			Path:         game.Path,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      game.CreatedAt,
			HasPath:      game.Path != "",
		})
	}
	return previews, nil
}

// Import 导入 LunaBox 数据库中的游戏与游玩记录。
func (l *LunaBoxImporter) Import(dbPath string, skipNoPath bool, samePathAction string) (ImportResult, error) {
	return l.ImportSelected(dbPath, skipNoPath, samePathAction, nil)
}

// ImportSelected 导入 LunaBox 数据库中被选中的游戏。
func (l *LunaBoxImporter) ImportSelected(dbPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)
	selectionFilter := newImportSelectionFilter(selections)

	games, sessionsByID, err := loadLunaBoxData(dbPath)
	if err != nil {
		applog.LogErrorf(l.deps.Ctx, "ImportFromLunaBox: 读取数据库失败: %v", err)
		return result, err
	}

	existingGames, existingNames, existingPaths, err := l.deps.existingGames("ImportFromLunaBox")
	if err != nil {
		return result, err
	}

	items := make([]ImportItem, 0, len(games))
	for _, game := range games {
		sessions := sessionsByID[game.ID]
		if game.Name == "" {
			result.Failed++
			result.FailedNames = append(result.FailedNames, fmt.Sprintf("LunaBox #%s (缺少名称)", game.ID))
			continue
		}
		if !selectionFilter.includes(game.Name, game.Path, string(game.SourceType), game.SourceID) {
			continue
		}
		if skipNoPath && game.Path == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, game.Name+" (无路径)")
			continue
		}

		action := ImportActionCreate
		existingGameID := ""
		if conflict, exists := findExistingGameConflict(existingGames, existingNames, existingPaths, game.Name, game.Path); exists {
			if conflict.Type != ConflictTypeSamePath || !IsSamePathMergeAction(samePathAction) {
				result.Skipped++
				if conflict.Type == ConflictTypeNameAndPath {
					result.SkippedNames = append(result.SkippedNames, game.Name+" (已存在)")
				} else {
					result.SkippedNames = append(result.SkippedNames, game.Name+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			if samePathAction == SamePathActionMergeSessions {
				action = ImportActionMergeSessions
			}
			existingGameID = conflict.Game.ID
			game.ID = conflict.Game.ID
			game.Path = conflict.Game.Path
			for i := range sessions {
				sessions[i].GameID = conflict.Game.ID
			}
		}

		items = append(items, ImportItem{
			Source:         vo.GameMetadataFromWebVO{Source: game.SourceType, Game: game},
			Sessions:       sessions,
			DisplayName:    game.Name,
			Path:           game.Path,
			Action:         action,
			ExistingGameID: existingGameID,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, game.Name, game.Path)
		}
	}

	batchResult, err := addImportedItems(l.deps, items)
	if err != nil {
		applog.LogErrorf(l.deps.Ctx, "ImportFromLunaBox: 批量写入失败: %v", err)
		return result, err
	}
	result.Success += batchResult.Success
	result.Skipped += batchResult.Skipped
	result.Failed += batchResult.Failed
	result.SessionsImported += batchResult.SessionsImported
	result.SkippedNames = append(result.SkippedNames, batchResult.SkippedNames...)
	result.FailedNames = append(result.FailedNames, batchResult.FailedNames...)
	return result, nil
}

// loadLunaBoxData 打开 LunaBox 数据库并读出游戏与按 game_id 分组的游玩记录。
func loadLunaBoxData(dbPath string) ([]models.Game, map[string][]models.PlaySession, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, nil, errors.New("LunaBox 数据库路径为空")
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 LunaBox 数据库失败: %w", err)
	}
	if info.IsDir() {
		return nil, nil, errors.New("LunaBox 数据库路径不能是目录")
	}

	// LunaBox 与本仓库用的是同一个 DuckDB 版本（duckdb-go/v2 v2.5.6），
	// 存储格式兼容，可以直接打开；改建成副本也不会动到原文件。
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 LunaBox 数据库失败: %w", err)
	}
	defer func() { _ = db.Close() }()

	games, err := readLunaBoxGames(db)
	if err != nil {
		return nil, nil, err
	}
	sessions, err := readLunaBoxSessions(db)
	if err != nil {
		return nil, nil, err
	}
	return games, sessions, nil
}

// readLunaBoxGames 读取 games 表。按列名取值，不假设列顺序与列数。
func readLunaBoxGames(db *sql.DB) ([]models.Game, error) {
	rows, columns, targets, err := startLunaBoxScan(db, "games")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	games := make([]models.Game, 0)
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("解析 LunaBox games 行失败: %w", err)
		}
		game := lunaBoxGameFromRow(lunaBoxRowValues(columns, targets))
		if game.ID == "" || game.Name == "" {
			continue
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 LunaBox games 失败: %w", err)
	}
	return games, nil
}

// readLunaBoxSessions 读取 play_sessions 表并按 game_id 分组。
func readLunaBoxSessions(db *sql.DB) (map[string][]models.PlaySession, error) {
	rows, columns, targets, err := startLunaBoxScan(db, "play_sessions")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	sessions := make(map[string][]models.PlaySession)
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("解析 LunaBox play_sessions 行失败: %w", err)
		}
		row := lunaBoxRowValues(columns, targets)
		session := models.PlaySession{
			ID:        lunaBoxString(row, "id"),
			GameID:    lunaBoxString(row, "game_id"),
			StartTime: lunaBoxTime(row, "start_time"),
			EndTime:   lunaBoxTime(row, "end_time"),
			// 桌面端库内 duration 单位是秒（见 docs/mobile-yukihub-migration.md），
			// LunaBox 同为桌面端，直接透传，不做毫秒换算。
			Duration:  int(lunaBoxFloat(row, "duration")),
			UpdatedAt: lunaBoxTime(row, "updated_at"),
		}
		if session.GameID == "" {
			continue
		}
		if session.UpdatedAt.IsZero() {
			session.UpdatedAt = session.StartTime
		}
		sessions[session.GameID] = append(sessions[session.GameID], session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 LunaBox play_sessions 失败: %w", err)
	}
	return sessions, nil
}

// startLunaBoxScan 打开一张表的全表查询并准备好按列扫描的容器。
func startLunaBoxScan(db *sql.DB, table string) (*sql.Rows, []string, []any, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("读取 LunaBox %s 表失败: %w", table, err)
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, nil, nil, fmt.Errorf("读取 LunaBox %s 列名失败: %w", table, err)
	}
	targets := make([]any, len(columns))
	for i := range targets {
		targets[i] = new(any)
	}
	return rows, columns, targets, nil
}

func lunaBoxRowValues(columns []string, targets []any) map[string]any {
	row := make(map[string]any, len(columns))
	for i, name := range columns {
		pointer, ok := targets[i].(*any)
		if !ok {
			continue
		}
		row[name] = *pointer
	}
	return row
}

func lunaBoxGameFromRow(row map[string]any) models.Game {
	now := time.Now()
	game := models.Game{
		ID:                 lunaBoxString(row, "id"),
		Name:               lunaBoxString(row, "name"),
		CoverURL:           lunaBoxString(row, "cover_url"),
		CoverSourceURL:     lunaBoxString(row, "cover_source_url"),
		Company:            lunaBoxString(row, "company"),
		Summary:            lunaBoxString(row, "summary"),
		Rating:             lunaBoxFloat(row, "rating"),
		ReleaseDate:        lunaBoxString(row, "release_date"),
		Path:               normalizeImportPath(lunaBoxString(row, "path")),
		GameDirectory:      lunaBoxString(row, "game_directory"),
		SavePath:           lunaBoxString(row, "save_path"),
		ProcessName:        lunaBoxString(row, "process_name"),
		LaunchMode:         enums.LaunchMode(lunaBoxString(row, "launch_mode")),
		SteamLaunchID:      lunaBoxString(row, "steam_launch_id"),
		SteamLaunchKind:    lunaBoxString(row, "steam_launch_kind"),
		SteamUserID:        lunaBoxString(row, "steam_user_id"),
		SteamLaunchOptions: lunaBoxString(row, "steam_launch_options"),
		Status:             mapLunaBoxStatus(lunaBoxString(row, "status")),
		SourceType:         lunaBoxSourceType(lunaBoxString(row, "source_type")),
		CachedAt:           lunaBoxTime(row, "cached_at"),
		SourceID:           lunaBoxString(row, "source_id"),
		CreatedAt:          lunaBoxTime(row, "created_at"),
		UpdatedAt:          lunaBoxTime(row, "updated_at"),
		UseLocaleEmulator:  lunaBoxBool(row, "use_locale_emulator"),
		UseMagpie:          lunaBoxBool(row, "use_magpie"),
		IsNSFW:             lunaBoxBool(row, "is_nsfw"),
		MetadataLocked:     lunaBoxBool(row, "metadata_locked"),
		LastPlayedAt:       lunaBoxTimePtr(row, "last_played_at"),
	}
	if game.CreatedAt.IsZero() {
		game.CreatedAt = now
	}
	if game.UpdatedAt.IsZero() {
		game.UpdatedAt = game.CreatedAt
	}
	return game
}

// mapLunaBoxStatus 把 LunaBox 的游玩状态换成 YukiHub 的五态。
//
// LunaBox 的 not_started 与 want_to_play 都并入 unplayed（手机版没有「想玩」这一态），
// on_hold 写作 onhold。不做这层映射不会报错，只会静默归成「未玩」——
// 表现为「搁置 / 在玩的游戏导入后全都变成未玩」。
func mapLunaBoxStatus(status string) enums.GameStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case string(enums.StatusPlaying), "in_progress":
		return enums.StatusPlaying
	case string(enums.StatusCompleted), "finished":
		return enums.StatusCompleted
	case string(enums.StatusOnHold), "on_hold":
		return enums.StatusOnHold
	case string(enums.StatusDropped), "abandoned":
		return enums.StatusDropped
	default:
		// not_started / want_to_play / unplayed / 空值 / 未知值
		return enums.StatusUnplayed
	}
}

// LunaBox 的来源取值是本仓库的子集（它 9 个，我们多了 bangumi_mirror 与 nextmoe），
// 所以直接透传即可；未知值兜底为 local。
var lunaBoxKnownSources = map[enums.SourceType]struct{}{
	enums.Local:        {},
	enums.Bangumi:      {},
	enums.VNDB:         {},
	enums.Ymgal:        {},
	enums.Steam:        {},
	enums.DLsite:       {},
	enums.TouchGal:     {},
	enums.Hikarinagi:   {},
	enums.ErogameScape: {},
	// 本仓库比 LunaBox 多出来的两个来源：读本仓库自己的库文件时会遇到，
	// 同样透传（按列名取值的设计本来就允许两边互为备份）。
	enums.BangumiMirror: {},
	enums.NextMoe:       {},
}

func lunaBoxSourceType(raw string) enums.SourceType {
	source := enums.SourceType(strings.ToLower(strings.TrimSpace(raw)))
	if _, ok := lunaBoxKnownSources[source]; ok {
		return source
	}
	return enums.Local
}

func lunaBoxString(row map[string]any, key string) string {
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func lunaBoxFloat(row map[string]any, key string) float64 {
	value, ok := row[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case int:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func lunaBoxBool(row map[string]any, key string) bool {
	value, ok := row[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		return typed != 0
	case int32:
		return typed != 0
	case string:
		lowered := strings.ToLower(strings.TrimSpace(typed))
		return lowered == "true" || lowered == "1"
	default:
		return false
	}
}

func lunaBoxTime(row map[string]any, key string) time.Time {
	value, ok := row[key]
	if !ok || value == nil {
		return time.Time{}
	}
	switch typed := value.(type) {
	case time.Time:
		return typed
	case string:
		trimmed := strings.TrimSpace(typed)
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999",
			"2006-01-02 15:04:05",
			"2006-01-02",
		} {
			if parsed, err := time.Parse(layout, trimmed); err == nil {
				return parsed
			}
		}
		return time.Time{}
	case int64:
		return time.Unix(typed, 0)
	default:
		return time.Time{}
	}
}

func lunaBoxTimePtr(row map[string]any, key string) *time.Time {
	parsed := lunaBoxTime(row, key)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}
