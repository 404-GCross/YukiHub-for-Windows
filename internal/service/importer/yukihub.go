package importer

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"yukihub/internal/applog"
	"yukihub/internal/common/enums"
	"yukihub/internal/common/vo"
	"yukihub/internal/models"
	"yukihub/internal/models/yukihub"

	"github.com/google/uuid"
)

const yukiHubMaxJSONSize = 64 << 20

var (
	yukiHubRatingPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)
	yukiHubTagSeparator  = regexp.MustCompile(`\s{2,}|[,，;；\r\n]+`)
)

type YukiHubImporter struct {
	deps Dependencies
}

type parsedYukiHubMetadata struct {
	cache    yukihub.MetadataCache
	data     yukihub.Metadata
	source   enums.SourceType
	sourceID string
}

func NewYukiHubImporter(deps Dependencies) *YukiHubImporter {
	return &YukiHubImporter{deps: deps}
}

func (y *YukiHubImporter) Preview(backupPath string) ([]PreviewGame, error) {
	backup, err := loadYukiHubBackup(backupPath)
	if err != nil {
		applog.LogErrorf(y.deps.Ctx, "PreviewYukiHubImport: failed to load backup: %v", err)
		return nil, err
	}

	existingGames, _, _, err := y.deps.existingGames("PreviewYukiHubImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)
	existingByName := make(map[string]models.Game, len(existingGames))
	for _, game := range existingGames {
		if key := strings.ToLower(strings.TrimSpace(game.Name)); key != "" {
			existingByName[key] = game
		}
	}

	metadataByGame := indexYukiHubMetadata(backup.MetadataCache)
	previews := make([]PreviewGame, 0, len(backup.Games))
	for _, sourceGame := range backup.Games {
		name := strings.TrimSpace(sourceGame.Title)
		if name == "" {
			continue
		}
		primary := pickYukiHubMetadata(metadataByGame[sourceGame.LocalID], backup.Settings.MetadataSource)
		sourceType, sourceID := yukiHubIdentity(primary)
		conflict := previewConflict(existingIndex, name, "", string(sourceType), sourceID)
		if conflict.Type == ConflictTypeNone {
			if existing, ok := existingByName[strings.ToLower(name)]; ok {
				conflict = existingGameConflict{Type: ConflictTypeNameAndPath, Game: existing}
			}
		}

		previews = append(previews, PreviewGame{
			Name:         name,
			Developer:    strings.TrimSpace(primary.data.Developer),
			SourceType:   string(sourceType),
			SourceID:     sourceID,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      yukiHubTimeOrNow(sourceGame.CreatedAt),
			HasPath:      false,
		})
	}

	return previews, nil
}

func (y *YukiHubImporter) Import(backupPath string, skipNoPath bool, samePathAction string) (ImportResult, error) {
	return y.ImportSelected(backupPath, skipNoPath, samePathAction, nil)
}

func (y *YukiHubImporter) ImportSelected(backupPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (ImportResult, error) {
	result := newImportResult()
	selectionFilter := newImportSelectionFilter(selections)
	samePathAction = NormalizeSamePathAction(samePathAction)

	backup, err := loadYukiHubBackup(backupPath)
	if err != nil {
		applog.LogErrorf(y.deps.Ctx, "ImportFromYukiHub: failed to load backup: %v", err)
		return result, err
	}

	existingGames, existingNames, existingPaths, err := y.deps.existingGames("ImportFromYukiHub")
	if err != nil {
		return result, err
	}

	metadataByGame := indexYukiHubMetadata(backup.MetadataCache)
	sessionsByGame := indexYukiHubSessions(backup.PlaySessions)
	items := make([]ImportItem, 0, len(backup.Games))
	for _, sourceGame := range backup.Games {
		gameName := strings.TrimSpace(sourceGame.Title)
		if gameName == "" {
			continue
		}

		metadataItems := metadataByGame[sourceGame.LocalID]
		primary := pickYukiHubMetadata(metadataItems, backup.Settings.MetadataSource)
		sourceType, sourceID := yukiHubIdentity(primary)
		if !selectionFilter.includes(gameName, "", string(sourceType), sourceID) {
			continue
		}
		if skipNoPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		// 与 PotatoVN 导入器对称：命中已有条目时按用户选择跳过或合并。
		// 合并会话是「总时长取最大值」的落地路径——两端会话取并集去重后，
		// 聚合补偿机制会把 total_play_time 与已录会话的差额补成一条聚合会话。
		action := ImportActionCreate
		existingGameID := ""
		conflict, conflictExists := findExistingGameConflict(existingGames, existingNames, existingPaths, gameName, "")
		if !conflictExists {
			// 与 Android 侧 findByTitleForEmptyRoot 对齐：快照条目没有路径，
			// 纯标题命中即视为同一条目（即使桌面端游戏带有本机路径）。
			// 通用判定的 NameAndPath 分支要求路径相等，会漏掉这种情况并产生重复条目。
			if existingID, ok := existingNames[strings.ToLower(gameName)]; ok {
				for _, game := range existingGames {
					if game.ID == existingID {
						conflict = existingGameConflict{Type: ConflictTypeNameAndPath, Game: game}
						conflictExists = true
						break
					}
				}
			}
		}
		if conflictExists {
			mergeable := conflict.Type == ConflictTypeSamePath || conflict.Type == ConflictTypeNameAndPath
			if !mergeable || !IsSamePathMergeAction(samePathAction) {
				result.Skipped++
				if conflict.Type == ConflictTypeNameAndPath {
					result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
				} else {
					result.SkippedNames = append(result.SkippedNames, gameName+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			if samePathAction == SamePathActionMergeSessions {
				action = ImportActionMergeSessions
			}
			existingGameID = conflict.Game.ID
		}

		game, sessions, tags := convertYukiHubGame(sourceGame, metadataItems, primary, sessionsByGame[sourceGame.LocalID], backup.CreatedAt)
		if TargetsExistingGame(action) {
			game.ID = existingGameID
			for i := range sessions {
				sessions[i].GameID = existingGameID
			}
		}
		items = append(items, ImportItem{
			Source: vo.GameMetadataFromWebVO{
				Source: game.SourceType,
				Game:   game,
				Tags:   tagsFromNames(tags),
			},
			Sessions:       sessions,
			DisplayName:    gameName,
			Action:         action,
			ExistingGameID: existingGameID,
			// 收藏在桌面端由「收藏」系统分类承载，models.Game 里没有对应列。
			Favorite: sourceGame.Favorite,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, gameName, "")
			existingGames = append(existingGames, game)
		}
	}

	batchResult, err := addImportedItems(y.deps, items)
	if err != nil {
		applog.LogErrorf(y.deps.Ctx, "ImportFromYukiHub: failed to batch add games: %v", err)
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

func loadYukiHubBackup(backupPath string) (*yukihub.Backup, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取 YukiHub 备份文件: %w", err)
	}
	defer file.Close()

	buffered := bufio.NewReader(file)
	header, err := buffered.Peek(2)
	if err != nil {
		return nil, fmt.Errorf("YukiHub 备份文件内容为空或已损坏: %w", err)
	}

	var reader io.Reader = buffered
	if header[0] == 0x1f && header[1] == 0x8b {
		gzipReader, gzipErr := gzip.NewReader(buffered)
		if gzipErr != nil {
			return nil, fmt.Errorf("解压 YukiHub 备份文件失败: %w", gzipErr)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	data, err := io.ReadAll(io.LimitReader(reader, yukiHubMaxJSONSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取 YukiHub 备份内容失败: %w", err)
	}
	if len(data) > yukiHubMaxJSONSize {
		return nil, fmt.Errorf("YukiHub 备份解压后超过 %d MiB", yukiHubMaxJSONSize>>20)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})

	var backup yukihub.Backup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("解析 YukiHub 备份文件失败: %w", err)
	}
	if backup.App != "YukiHub" {
		return nil, fmt.Errorf("所选文件不是有效的 YukiHub 备份")
	}
	return &backup, nil
}

func indexYukiHubMetadata(entries []yukihub.MetadataCache) map[int64][]parsedYukiHubMetadata {
	result := make(map[int64][]parsedYukiHubMetadata)
	for _, entry := range entries {
		parsed := parsedYukiHubMetadata{
			cache:    entry,
			source:   mapYukiHubSourceType(entry.Source),
			sourceID: strings.TrimSpace(entry.SourceID),
		}
		_ = json.Unmarshal([]byte(entry.JSON), &parsed.data)
		if parsed.sourceID == "" {
			parsed.sourceID = strings.TrimSpace(parsed.data.ID)
		}
		result[entry.GameLocalID] = append(result[entry.GameLocalID], parsed)
	}
	return result
}

func indexYukiHubSessions(entries []yukihub.PlaySession) map[int64][]yukihub.PlaySession {
	result := make(map[int64][]yukihub.PlaySession)
	for _, entry := range entries {
		result[entry.GameLocalID] = append(result[entry.GameLocalID], entry)
	}
	return result
}

func pickYukiHubMetadata(items []parsedYukiHubMetadata, preferredSource string) parsedYukiHubMetadata {
	preferred := mapYukiHubSourceType(preferredSource)
	if preferred != enums.Local {
		for _, item := range items {
			if item.source == preferred && item.sourceID != "" {
				return item
			}
		}
	}
	for _, sourceType := range yukiHubFallbackSourceOrder {
		for _, item := range items {
			if item.source == sourceType && item.sourceID != "" {
				return item
			}
		}
	}
	if len(items) > 0 {
		return items[0]
	}
	return parsedYukiHubMetadata{source: enums.Local}
}

func yukiHubIdentity(metadata parsedYukiHubMetadata) (enums.SourceType, string) {
	if metadata.source == "" || metadata.sourceID == "" {
		return enums.Local, ""
	}
	return metadata.source, metadata.sourceID
}

// mapYukiHubSourceType 把备份里的来源名映射成桌面端的来源枚举。
//
// **名单必须涵盖手机端支持的全部来源**。早先这里漏了 nextmoe：手机版的
// `settings.metadata_source` 可以就是 nextmoe，映射不出来就变成 `local`，
// 于是「偏好来源」判定被整体跳过（见 pickYukiHubMetadata 里 `preferred != Local`
// 的守卫），最后落到兜底名单的 vndb 上——用户看到的现象就是
// 「好几个本来是 nextmoe 源的游戏，导进来变成了 vndb」。
func mapYukiHubSourceType(source string) enums.SourceType {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case string(enums.VNDB):
		return enums.VNDB
	case string(enums.Bangumi), "bangumi_mirror":
		return enums.Bangumi
	case string(enums.Ymgal):
		return enums.Ymgal
	case string(enums.Hikarinagi):
		return enums.Hikarinagi
	case string(enums.NextMoe):
		return enums.NextMoe
	// 桌面端认得但手机端目前不产的来源，一并保留身份，
	// 免得将来对端新增来源时又静默退化成 local。
	case string(enums.Steam):
		return enums.Steam
	case string(enums.DLsite):
		return enums.DLsite
	case string(enums.TouchGal):
		return enums.TouchGal
	case string(enums.ErogameScape):
		return enums.ErogameScape
	default:
		return enums.Local
	}
}

// yukiHubFallbackSourceOrder 是「备份里没有可用的偏好来源」时的兜底优先级。
//
// 同样必须涵盖手机端支持的全部来源，否则漏掉的那个来源的游戏会被错误地
// 归到名单里靠前的来源上。
var yukiHubFallbackSourceOrder = []enums.SourceType{
	enums.VNDB,
	enums.Bangumi,
	enums.Ymgal,
	enums.Hikarinagi,
	enums.NextMoe,
}

func convertYukiHubGame(
	source yukihub.Game,
	metadataItems []parsedYukiHubMetadata,
	primary parsedYukiHubMetadata,
	sourceSessions []yukihub.PlaySession,
	backupCreatedAt int64,
) (models.Game, []models.PlaySession, []string) {
	gameID := uuid.New().String()
	createdAt := yukiHubTime(source.CreatedAt)
	if createdAt.IsZero() {
		createdAt = yukiHubTime(backupCreatedAt)
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := yukiHubTime(source.UpdatedAt)
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	sourceType, sourceID := yukiHubIdentity(primary)
	coverURL := strings.TrimSpace(primary.data.CoverURL)
	if !strings.HasPrefix(coverURL, "https://") && !strings.HasPrefix(coverURL, "http://") {
		coverURL = ""
	}
	// 清零时间为 Unix 毫秒，0 表示从未清零，必须落成 NULL 而不是 1970 年时间戳。
	var playtimeResetAt *time.Time
	if resetAt := yukiHubTime(source.PlaytimeResetAt); !resetAt.IsZero() {
		playtimeResetAt = &resetAt
	}
	game := models.Game{
		ID:              gameID,
		Name:            strings.TrimSpace(source.Title),
		Aliases:         yukiHubAliases(source.Title, source.OriginalTitle, primary.data.ChineseTitle, primary.data.OriginalTitle, primary.data.RomanTitle),
		CoverURL:        coverURL,
		CoverSourceURL:  coverURL,
		Company:         strings.TrimSpace(primary.data.Developer),
		Summary:         firstYukiHubString(source.Description, primary.data.TranslatedDescription, primary.data.Description),
		Rating:          parseYukiHubRating(primary.data.RatingText),
		ReleaseDate:     strings.TrimSpace(primary.data.Released),
		Status:          mapYukiHubGameStatus(source.PlayStatus),
		IsNSFW:          source.NSFW,
		SourceType:      sourceType,
		SourceID:        sourceID,
		MetadataSources: collectYukiHubMetadataSources(metadataItems, updatedAt),
		CachedAt:        updatedAt,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		LegacyLocalID:   yukiHubLocalID(source.LocalID),
		// Android 备份的 profile 里没有可用的设备标识，保持空值，不编造来源设备。
		SourceDeviceID:  "",
		PlaytimeResetAt: playtimeResetAt,
		Hidden:          source.Hidden,
	}

	tags := parseYukiHubTags(source.Tags)
	for _, item := range metadataItems {
		tags = append(tags, parseYukiHubTags(item.data.TagsText)...)
	}
	tags = uniqueYukiHubStrings(tags...)
	sessions := convertYukiHubSessions(gameID, source, sourceSessions, createdAt, updatedAt)
	return game, sessions, tags
}

func collectYukiHubMetadataSources(items []parsedYukiHubMetadata, fallbackTime time.Time) []models.GameMetadataSource {
	bySource := make(map[enums.SourceType]models.GameMetadataSource)
	for _, item := range items {
		if item.source == enums.Local || item.sourceID == "" {
			continue
		}
		cachedAt := yukiHubTime(item.cache.UpdatedAt)
		if cachedAt.IsZero() {
			cachedAt = fallbackTime
		}
		bySource[item.source] = models.GameMetadataSource{
			SourceType: item.source,
			SourceID:   item.sourceID,
			// 原样保留 Android 的 VnMetadata 负载：桌面端只解析出自己用得到的字段，
			// 直接丢弃会让「Android → 桌面端 → Android」的往返丢掉截图、罗马音标题等
			// 桌面端没有对应列的字段。
			CacheJSON: strings.TrimSpace(item.cache.JSON),
			CachedAt:  cachedAt,
			UpdatedAt: cachedAt,
		}
	}
	result := make([]models.GameMetadataSource, 0, len(bySource))
	for _, item := range bySource {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceType < result[j].SourceType })
	return result
}

func convertYukiHubSessions(gameID string, game yukihub.Game, entries []yukihub.PlaySession, createdAt time.Time, updatedAt time.Time) []models.PlaySession {
	sessions := make([]models.PlaySession, 0, len(entries)+1)
	recordedDuration := 0
	var earliestStart time.Time
	for index, entry := range entries {
		startTime := yukiHubTime(entry.StartTime)
		endTime := yukiHubTime(entry.EndTime)
		durationMillis := entry.Duration
		if durationMillis <= 0 && !startTime.IsZero() && !endTime.IsZero() {
			durationMillis = endTime.Sub(startTime).Milliseconds()
		}
		if durationMillis <= 0 {
			continue
		}
		durationSeconds := int(durationMillis / 1000)
		if durationSeconds == 0 {
			durationSeconds = 1
		}
		if startTime.IsZero() && !endTime.IsZero() {
			startTime = endTime.Add(-time.Duration(durationSeconds) * time.Second)
		}
		if startTime.IsZero() {
			continue
		}
		if endTime.IsZero() || !endTime.After(startTime) {
			endTime = startTime.Add(time.Duration(durationSeconds) * time.Second)
		}
		sessionUpdatedAt := yukiHubTime(entry.UpdatedAt)
		if sessionUpdatedAt.IsZero() {
			sessionUpdatedAt = endTime
		}
		sessionID := strings.TrimSpace(entry.SessionUUID)
		if _, err := uuid.Parse(sessionID); err != nil {
			sessionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("yukihub:%d:%d:%d:%d", game.LocalID, entry.StartTime, entry.EndTime, index))).String()
		}
		sessions = append(sessions, models.PlaySession{
			ID:        sessionID,
			GameID:    gameID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  durationSeconds,
			UpdatedAt: sessionUpdatedAt,
		})
		recordedDuration += durationSeconds
		if earliestStart.IsZero() || startTime.Before(earliestStart) {
			earliestStart = startTime
		}
	}

	totalDuration := int(game.TotalPlayTime / 1000)
	if totalDuration > recordedDuration {
		remainder := totalDuration - recordedDuration
		endTime := earliestStart
		if endTime.IsZero() {
			endTime = yukiHubTime(game.LastPlayedAt)
		}
		if endTime.IsZero() {
			endTime = createdAt
		}
		sessions = append(sessions, models.PlaySession{
			ID:        uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("yukihub:%d:aggregate", game.LocalID))).String(),
			GameID:    gameID,
			StartTime: endTime.Add(-time.Duration(remainder) * time.Second),
			EndTime:   endTime,
			Duration:  remainder,
			UpdatedAt: updatedAt,
		})
	}
	return sessions
}

// mapYukiHubGameStatus 把 YukiHub 备份里的 play_status 映射为桌面端状态。
//
// 两端状态已经是同一套五态字符串：unplayed 未玩 / playing 在玩 /
// completed 玩过 / onhold 搁置 / dropped 抛弃。这里额外容忍下划线、过去式等
// 历史写法，避免旧备份解析不到；unplayed 与未知值一律落到「未玩」。
func mapYukiHubGameStatus(status string) enums.GameStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "playing":
		return enums.StatusPlaying
	case "completed":
		return enums.StatusCompleted
	case "onhold", "on_hold", "on-hold", "shelved", "paused", "hold":
		return enums.StatusOnHold
	case "dropped", "drop", "abandoned", "abandon", "give_up":
		return enums.StatusDropped
	default:
		return enums.StatusUnplayed
	}
}

func parseYukiHubRating(raw string) float64 {
	match := yukiHubRatingPattern.FindString(raw)
	if match == "" {
		return 0
	}
	rating, err := strconv.ParseFloat(match, 64)
	if err != nil || rating < 0 || rating > 10 {
		return 0
	}
	return rating
}

func parseYukiHubTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return uniqueYukiHubStrings(yukiHubTagSeparator.Split(raw, -1)...)
}

func uniqueYukiHubStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func yukiHubAliases(gameName string, values ...string) []string {
	aliases := uniqueYukiHubStrings(values...)
	result := aliases[:0]
	for _, alias := range aliases {
		if !strings.EqualFold(strings.TrimSpace(gameName), alias) {
			result = append(result, alias)
		}
	}
	return result
}

func firstYukiHubString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// yukiHubLocalID 把 Android 侧的整数 local_id 保留为十进制字符串，0 视为缺失。
func yukiHubLocalID(localID int64) string {
	if localID <= 0 {
		return ""
	}
	return strconv.FormatInt(localID, 10)
}

func yukiHubTime(milliseconds int64) time.Time {
	if milliseconds <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(milliseconds)
}

func yukiHubTimeOrNow(milliseconds int64) time.Time {
	value := yukiHubTime(milliseconds)
	if value.IsZero() {
		return time.Now()
	}
	return value
}
