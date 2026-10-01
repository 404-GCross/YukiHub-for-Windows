package importer

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"yukihub/internal/applog"
	"yukihub/internal/common/enums"
	"yukihub/internal/common/vo"
	"yukihub/internal/models"
	"yukihub/internal/utils/imageutils"
	"yukihub/internal/utils/metadata"
)

// LunaBoxImporter 从上游 LunaBox 的 ZIP 备份导入游戏、游玩记录与标签。
//
// LunaBox 的备份是 **ZIP**（默认落在
// `%APPDATA%\LunaBox\backups\database\lunabox_<时间戳>.zip`），
// 里面是 DuckDB 导出的 CSV 与封面图片：
//
//	database/games.csv            游戏列表
//	database/play_sessions.csv    游玩记录（duration 单位：秒）
//	database/game_tags.csv        标签（每个游戏多条）
//	covers/<游戏 ID>.webp         封面，对应 games.cover_url 里的 /local/covers/<文件名>
//
// 只依赖 CSV 的**列名**取值，所以 LunaBox 之后增删列都不会影响解析；
// 也正因为读的是导出文件而不是数据库，导入时不会碰到对方库的写锁。
//
// 与手机版（`LunaBoxImporter.java`）的差异：桌面端 path 是有效的 Windows 路径，
// 因此**保留启动路径**（手机版因为路径不可达而丢弃），游玩记录与封面同样照搬。
type LunaBoxImporter struct {
	deps Dependencies
}

// NewLunaBoxImporter 创建 LunaBox 导入器。
func NewLunaBoxImporter(deps Dependencies) *LunaBoxImporter {
	return &LunaBoxImporter{deps: deps}
}

const (
	lunaBoxGamesFileName    = "games.csv"
	lunaBoxSessionsFileName = "play_sessions.csv"
	lunaBoxTagsFileName     = "game_tags.csv"
	// LunaBox 的本地封面统一写成 /local/covers/<文件名>，图片本体在 ZIP 的 covers/ 下。
	lunaBoxLocalCoverPrefix = "/local/covers/"
	// 与手机版一致：每个游戏最多带 20 个标签，避免标签爆炸。
	lunaBoxMaxTagsPerGame = 20
	// 单个 ZIP 条目的读取上限，纯属防呆（正常备份的 CSV 只有几十 KB）。
	lunaBoxMaxEntryBytes = 64 << 20
)

// lunaBoxEntry 是一条待导入的游戏，附带只在导入期需要的旁路信息。
type lunaBoxEntry struct {
	game models.Game
	tags []metadata.TagItem
	// lunaBoxID 是 LunaBox 侧的 id。主键（game.ID）由本机重新生成，
	// 因此游玩记录、标签、封面都必须靠这个 id 关联 —— 不能拿 game.ID 去查。
	lunaBoxID string
	// coverFile 是 ZIP 里 covers/ 下的文件名（games.cover_url 为本地封面时才有值）。
	coverFile string
}

// lunaBoxArchive 是一次解析出来的完整备份内容。
type lunaBoxArchive struct {
	entries  []lunaBoxEntry
	sessions map[string][]models.PlaySession // LunaBox 游戏 ID -> 游玩记录
	covers   map[string][]byte               // covers/ 下的文件名 -> 图片内容
}

// Preview 预览 LunaBox 备份中的游戏。
func (l *LunaBoxImporter) Preview(zipPath string) ([]PreviewGame, error) {
	archive, err := readLunaBoxArchive(zipPath)
	if err != nil {
		applog.LogErrorf(l.deps.Ctx, "PreviewLunaBoxImport: 读取备份失败: %v", err)
		return nil, err
	}

	existingGames, _, _, err := l.deps.existingGames("PreviewLunaBoxImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(archive.entries))
	for _, entry := range archive.entries {
		game := entry.game
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

// Import 导入 LunaBox 备份中的全部游戏与游玩记录。
func (l *LunaBoxImporter) Import(zipPath string, skipNoPath bool, samePathAction string) (ImportResult, error) {
	return l.ImportSelected(zipPath, skipNoPath, samePathAction, nil)
}

// ImportSelected 只导入被选中的条目。
func (l *LunaBoxImporter) ImportSelected(zipPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)
	selectionFilter := newImportSelectionFilter(selections)

	archive, err := readLunaBoxArchive(zipPath)
	if err != nil {
		applog.LogErrorf(l.deps.Ctx, "ImportFromLunaBox: 读取备份失败: %v", err)
		return result, err
	}

	existingGames, existingNames, existingPaths, err := l.deps.existingGames("ImportFromLunaBox")
	if err != nil {
		return result, err
	}

	items := make([]ImportItem, 0, len(archive.entries))
	for _, entry := range archive.entries {
		game := entry.game
		if game.Name == "" {
			result.Failed++
			result.FailedNames = append(result.FailedNames, "LunaBox 条目 (缺少名称)")
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

		// 游玩记录按 LunaBox 的 id 关联，并换成本机主键
		sessions := rekeyLunaBoxSessions(archive.sessions[entry.lunaBoxID], game.ID)
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
			Source:         vo.GameMetadataFromWebVO{Source: game.SourceType, Game: game, Tags: entry.tags},
			Sessions:       sessions,
			DisplayName:    game.Name,
			Path:           game.Path,
			Action:         action,
			ExistingGameID: existingGameID,
			CoverLoader:    l.coverLoader(entry.coverFile, archive.covers),
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

// rekeyLunaBoxSessions 复制一份游玩记录并把 GameID 换成本机主键。
//
// LunaBox 记录里的 game_id 是它自己的 uuid，与本机新生成的主键不同；
// 不换就会写出一批「指向不存在的游戏」的孤立记录（不报错，但游玩时长全丢）。
func rekeyLunaBoxSessions(sessions []models.PlaySession, gameID string) []models.PlaySession {
	if len(sessions) == 0 {
		return nil
	}
	cloned := make([]models.PlaySession, len(sessions))
	copy(cloned, sessions)
	for i := range cloned {
		cloned[i].GameID = gameID
	}
	return cloned
}

// coverLoader 把 ZIP 里的封面字节落成本地封面。
//
// 返回的闭包持有整份封面映射；ZIP 只有几百 KB（封面是缩略图级别），
// 放内存里可以避免「临时目录在异步落盘前就被清掉」的问题。
func (l *LunaBoxImporter) coverLoader(coverFile string, covers map[string][]byte) func(models.Game) (string, error) {
	if coverFile == "" {
		return nil
	}
	data := covers[coverFile]
	if len(data) == 0 {
		return nil
	}
	return func(game models.Game) (string, error) {
		savedPath, err := imageutils.SaveCoverImageBytes(data, game.ID, "image/webp")
		if err != nil {
			return "", fmt.Errorf("保存 LunaBox 封面失败: %w", err)
		}
		return savedPath, nil
	}
}

// readLunaBoxArchive 打开 ZIP 并解析出游戏、游玩记录与封面。
func readLunaBoxArchive(zipPath string) (*lunaBoxArchive, error) {
	zipPath = strings.TrimSpace(zipPath)
	if zipPath == "" {
		return nil, errors.New("LunaBox 备份路径为空")
	}
	info, err := os.Stat(zipPath)
	if err != nil {
		return nil, fmt.Errorf("读取 LunaBox 备份失败: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("LunaBox 备份路径不能是目录")
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("打开 LunaBox 备份失败（需要 .zip 备份文件）: %w", err)
	}
	defer func() { _ = reader.Close() }()

	var gamesCSV, sessionsCSV, tagsCSV []byte
	covers := make(map[string][]byte)

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(file.Name)
		switch {
		case matchesLunaBoxFileName(name, lunaBoxGamesFileName):
			if gamesCSV, err = readLunaBoxZipEntry(file); err != nil {
				return nil, err
			}
		case matchesLunaBoxFileName(name, lunaBoxSessionsFileName):
			if sessionsCSV, err = readLunaBoxZipEntry(file); err != nil {
				return nil, err
			}
		case matchesLunaBoxFileName(name, lunaBoxTagsFileName):
			if tagsCSV, err = readLunaBoxZipEntry(file); err != nil {
				return nil, err
			}
		case strings.HasPrefix(name, "covers/") && isLunaBoxImageFile(name):
			data, readErr := readLunaBoxZipEntry(file)
			if readErr != nil {
				return nil, readErr
			}
			covers[filepath.Base(name)] = data
		}
	}

	if len(gamesCSV) == 0 {
		return nil, fmt.Errorf("备份里没有找到 database/%s", lunaBoxGamesFileName)
	}

	archive := &lunaBoxArchive{
		sessions: parseLunaBoxSessions(sessionsCSV),
		covers:   covers,
	}
	tagsByGameID := parseLunaBoxTags(tagsCSV)
	archive.entries = parseLunaBoxGames(gamesCSV, tagsByGameID)
	if len(archive.entries) == 0 {
		return nil, errors.New("LunaBox 备份里没有游戏条目")
	}
	return archive, nil
}

// matchesLunaBoxFileName 兼容「database/xxx.csv」与「xxx.csv」两种打包方式，
// 也容忍大小写差异。
func matchesLunaBoxFileName(name string, fileName string) bool {
	return strings.EqualFold(filepath.Base(name), fileName)
}

func isLunaBoxImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".avif":
		return true
	default:
		return false
	}
}

func readLunaBoxZipEntry(file *zip.File) ([]byte, error) {
	opened, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("读取备份条目 %s 失败: %w", file.Name, err)
	}
	defer func() { _ = opened.Close() }()

	data, err := io.ReadAll(io.LimitReader(opened, lunaBoxMaxEntryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取备份条目 %s 失败: %w", file.Name, err)
	}
	if int64(len(data)) > lunaBoxMaxEntryBytes {
		return nil, fmt.Errorf("备份条目 %s 过大，已跳过", file.Name)
	}
	return data, nil
}

// parseLunaBoxGames 解析 games.csv。按列名取值，因此列的顺序与数量都不敏感。
func parseLunaBoxGames(data []byte, tagsByGameID map[string][]metadata.TagItem) []lunaBoxEntry {
	records, err := readLunaBoxCSV(data)
	if err != nil || len(records) < 2 {
		return nil
	}

	header := lunaBoxHeaderIndex(records[0])
	entries := make([]lunaBoxEntry, 0, len(records)-1)
	for _, record := range records[1:] {
		name := lunaBoxField(record, header, "name")
		if name == "" {
			continue
		}

		lunaBoxID := lunaBoxField(record, header, "id")
		game := models.Game{
			// 与其它导入器一致：主键由本机生成，LunaBox 的 id 只用于关联
			// 游玩记录、标签与封面文件。
			ID:                 uuid.New().String(),
			Name:               name,
			Aliases:            parseLunaBoxAliases(lunaBoxField(record, header, "aliases")),
			CoverSourceURL:     lunaBoxField(record, header, "cover_source_url"),
			Company:            lunaBoxField(record, header, "company"),
			Summary:            lunaBoxField(record, header, "summary"),
			Rating:             parseLunaBoxFloat(lunaBoxField(record, header, "rating")),
			ReleaseDate:        lunaBoxField(record, header, "release_date"),
			Path:               normalizeImportPath(lunaBoxField(record, header, "path")),
			GameDirectory:      lunaBoxField(record, header, "game_directory"),
			SavePath:           lunaBoxField(record, header, "save_path"),
			ProcessName:        lunaBoxField(record, header, "process_name"),
			LaunchMode:         enums.LaunchMode(lunaBoxField(record, header, "launch_mode")),
			SteamLaunchID:      lunaBoxField(record, header, "steam_launch_id"),
			SteamLaunchKind:    lunaBoxField(record, header, "steam_launch_kind"),
			SteamUserID:        lunaBoxField(record, header, "steam_user_id"),
			SteamLaunchOptions: lunaBoxField(record, header, "steam_launch_options"),
			Status:             mapLunaBoxStatus(lunaBoxField(record, header, "status")),
			// 来源名映射沿用 importer 里唯一的一份实现（source_mapping.go），
			// 不要在这里另写一个 switch：漏项的后果是静默归成 local。
			SourceType:        mapExternalSourceName(lunaBoxField(record, header, "source_type")),
			SourceID:          lunaBoxField(record, header, "source_id"),
			CachedAt:          parseLunaBoxTime(lunaBoxField(record, header, "cached_at")),
			CreatedAt:         parseLunaBoxTime(lunaBoxField(record, header, "created_at")),
			UpdatedAt:         parseLunaBoxTime(lunaBoxField(record, header, "updated_at")),
			UseLocaleEmulator: parseLunaBoxBool(lunaBoxField(record, header, "use_locale_emulator")),
			UseMagpie:         parseLunaBoxBool(lunaBoxField(record, header, "use_magpie")),
			IsNSFW:            parseLunaBoxBool(lunaBoxField(record, header, "is_nsfw")),
			MetadataLocked:    parseLunaBoxBool(lunaBoxField(record, header, "metadata_locked")),
			LastPlayedAt:      parseLunaBoxTimePtr(lunaBoxField(record, header, "last_played_at")),
		}
		now := time.Now()
		if game.CreatedAt.IsZero() {
			game.CreatedAt = now
		}
		if game.UpdatedAt.IsZero() {
			game.UpdatedAt = game.CreatedAt
		}

		entry := lunaBoxEntry{game: game, tags: tagsByGameID[lunaBoxID], lunaBoxID: lunaBoxID}
		entry.coverFile = lunaBoxLocalCoverFile(lunaBoxField(record, header, "cover_url"))
		if entry.coverFile == "" {
			// 远程封面（http/https）交给常规的封面下载流程。
			coverURL := lunaBoxField(record, header, "cover_url")
			if strings.HasPrefix(coverURL, "http://") || strings.HasPrefix(coverURL, "https://") {
				entry.game.CoverURL = coverURL
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

// lunaBoxLocalCoverFile 把 /local/covers/<文件名> 换成 ZIP 里的文件名。
func lunaBoxLocalCoverFile(coverURL string) string {
	if !strings.HasPrefix(coverURL, lunaBoxLocalCoverPrefix) {
		return ""
	}
	fileName := strings.TrimPrefix(coverURL, lunaBoxLocalCoverPrefix)
	fileName = strings.TrimSpace(filepath.Base(filepath.ToSlash(fileName)))
	if fileName == "" || fileName == "." {
		return ""
	}
	return fileName
}

// parseLunaBoxSessions 解析 play_sessions.csv 并按游戏分组。
func parseLunaBoxSessions(data []byte) map[string][]models.PlaySession {
	sessions := make(map[string][]models.PlaySession)
	if len(data) == 0 {
		return sessions
	}
	records, err := readLunaBoxCSV(data)
	if err != nil || len(records) < 2 {
		return sessions
	}

	header := lunaBoxHeaderIndex(records[0])
	for _, record := range records[1:] {
		lunaBoxGameID := lunaBoxField(record, header, "game_id")
		if lunaBoxGameID == "" {
			continue
		}
		startTime := parseLunaBoxTime(lunaBoxField(record, header, "start_time"))
		session := models.PlaySession{
			ID:        lunaBoxField(record, header, "id"),
			GameID:    lunaBoxGameID,
			StartTime: startTime,
			EndTime:   parseLunaBoxTime(lunaBoxField(record, header, "end_time")),
			// LunaBox 的 duration 单位是秒，与桌面端库内一致，直接透传。
			// （手机版的同步格式才是毫秒，从手机版导入时需 ×1000。）
			Duration:  parseLunaBoxInt(lunaBoxField(record, header, "duration")),
			UpdatedAt: parseLunaBoxTime(lunaBoxField(record, header, "updated_at")),
		}
		if session.UpdatedAt.IsZero() {
			session.UpdatedAt = session.StartTime
		}
		sessions[lunaBoxGameID] = append(sessions[lunaBoxGameID], session)
	}
	return sessions
}

// parseLunaBoxTags 解析 game_tags.csv，返回「LunaBox 游戏 ID -> 标签」。
func parseLunaBoxTags(data []byte) map[string][]metadata.TagItem {
	tagsByGameID := make(map[string][]metadata.TagItem)
	if len(data) == 0 {
		return tagsByGameID
	}
	records, err := readLunaBoxCSV(data)
	if err != nil || len(records) < 2 {
		return tagsByGameID
	}

	header := lunaBoxHeaderIndex(records[0])
	for _, record := range records[1:] {
		lunaBoxGameID := lunaBoxField(record, header, "game_id")
		name := lunaBoxField(record, header, "name")
		if lunaBoxGameID == "" || name == "" {
			continue
		}
		if len(tagsByGameID[lunaBoxGameID]) >= lunaBoxMaxTagsPerGame {
			continue
		}
		tagsByGameID[lunaBoxGameID] = append(tagsByGameID[lunaBoxGameID], metadata.TagItem{
			Name:      name,
			Source:    lunaBoxField(record, header, "source"),
			Weight:    parseLunaBoxFloat(lunaBoxField(record, header, "weight")),
			IsSpoiler: parseLunaBoxBool(lunaBoxField(record, header, "is_spoiler")),
		})
	}
	return tagsByGameID
}

// readLunaBoxCSV 解析 CSV。DuckDB 导出时字段可能含逗号与换行（简介就是），
// 都会被双引号包裹，标准库即可正确处理；首行的 BOM 需要手工去掉，
// 否则第一列的名字会带上 \ufeff 而取不到值。
func readLunaBoxCSV(data []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\ufeff"))))
	// 列数不固定时不报错（LunaBox 各版本列数不同）。
	reader.FieldsPerRecord = -1
	// 容忍个别未转义的引号，避免一条脏数据让整份备份导入失败。
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析 LunaBox CSV 失败: %w", err)
	}
	return records, nil
}

// lunaBoxHeaderIndex 把表头行变成「列名 -> 下标」。
func lunaBoxHeaderIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		if key == "" {
			continue
		}
		if _, exists := index[key]; !exists {
			index[key] = i
		}
	}
	return index
}

// lunaBoxField 按列名取值，缺列或越界一律返回空串 —— LunaBox 增删列都不会出错。
func lunaBoxField(record []string, header map[string]int, column string) string {
	index, ok := header[column]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
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

// parseLunaBoxAliases 解析别名列（DuckDB 导出的是 JSON 数组字面量，如 ["a","b"]）。
func parseLunaBoxAliases(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var aliases []string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return nil
	}
	cleaned := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if trimmed := strings.TrimSpace(alias); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

// parseLunaBoxTime 解析 LunaBox 的时间戳，形如 2026-07-16 11:56:43.468941+08
// （PostgreSQL 风格，带时区偏移；小数位数不定，偶有无时区的情况）。
func parseLunaBoxTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "null") {
		return time.Time{}
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05.999999999Z07:00",
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func parseLunaBoxTimePtr(raw string) *time.Time {
	parsed := parseLunaBoxTime(raw)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func parseLunaBoxFloat(raw string) float64 {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "null") {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseLunaBoxInt(raw string) int {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "null") {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int(parsed)
}

// parseLunaBoxBool 解析布尔列。DuckDB 导出的是小写 true / false，
// 这里同时容忍 t / 1 / yes；缺列或空值按 false，与 LunaBox 的 DEFAULT FALSE 一致。
func parseLunaBoxBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "t", "1", "yes":
		return true
	default:
		return false
	}
}
