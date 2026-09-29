package test

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
	"yukihub/internal/models"
	"yukihub/internal/models/yukihub"
	"yukihub/internal/service/exporter"
	"yukihub/internal/service/gamehelper"
	"yukihub/internal/service/importer"
)

func TestYukiHubExporterBuildFromEmptyDatabase(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	backup, err := exporter.NewYukiHubExporter(context.Background(), db).Build()
	if err != nil {
		t.Fatalf("空库导出失败: %v", err)
	}
	if backup.App != "YukiHub" || backup.Schema != 5 {
		t.Fatalf("快照头部 = %s/schema %d, want YukiHub/5", backup.App, backup.Schema)
	}
	if len(backup.Games) != 0 || len(backup.PlaySessions) != 0 || len(backup.MetadataCache) != 0 {
		t.Fatalf("空库导出应为空快照, got %d games / %d sessions", len(backup.Games), len(backup.PlaySessions))
	}
}

func TestYukiHubExporterBuildMapsDesktopLibrary(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	createdAt := time.Date(2026, time.September, 25, 10, 0, 0, 0, time.Local)
	resetAt := createdAt.Add(24 * time.Hour)
	if _, err := db.Exec(`
		INSERT INTO games (id, name, aliases, cover_url, summary, path, game_directory, status, source_type, source_id,
			created_at, updated_at, legacy_local_id, playtime_reset_at, hidden, is_nsfw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"game-1", "清零过的游戏", `["Reset Game"]`, "D:\\covers\\local.jpg", "简介",
		"D:\\Games\\one\\game.exe", "D:\\Games\\one", "playing", "local", "",
		createdAt, createdAt.Add(time.Hour), "42", resetAt, true, true); err != nil {
		t.Fatalf("插入游戏失败: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO games (id, name, cover_url, path, status, source_type, source_id, created_at, updated_at, legacy_local_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"game-2", "收藏的游戏", "https://example.com/cover.jpg", "D:\\Games\\two\\game.exe",
		"want_to_play", "local", "", createdAt, createdAt.Add(time.Hour), "not-a-number"); err != nil {
		t.Fatalf("插入游戏失败: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"tag-1", "game-2", "剧情", "vndb", 1.0, false, createdAt, createdAt); err != nil {
		t.Fatalf("插入标签失败: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"tag-2", "game-2", "校园", "vndb", 1.0, false, createdAt, createdAt); err != nil {
		t.Fatalf("插入标签失败: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO game_categories (game_id, category_id, updated_at) VALUES (?, ?, ?)`,
		"game-2", gamehelper.SystemFavoritesCategoryID, createdAt); err != nil {
		t.Fatalf("插入收藏分类失败: %v", err)
	}

	// 清零之前的会话（90 秒）不导出；清零之后的会话（90 秒）导出为 90000 毫秒。
	beforeStart := createdAt.Add(time.Hour)
	afterStart := resetAt.Add(time.Hour)
	for _, session := range []struct {
		id        string
		gameID    string
		startTime time.Time
		duration  int
	}{
		{"session-before", "game-1", beforeStart, 90},
		{"session-after", "game-1", afterStart, 90},
		{"session-favorite", "game-2", afterStart, 60},
	} {
		if _, err := db.Exec(
			`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			session.id, session.gameID, session.startTime, session.startTime.Add(time.Duration(session.duration)*time.Second),
			session.duration, session.startTime); err != nil {
			t.Fatalf("插入游玩记录失败: %v", err)
		}
	}

	backup, err := exporter.NewYukiHubExporter(context.Background(), db).Build()
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if len(backup.Games) != 2 {
		t.Fatalf("导出游戏数 = %d, want 2", len(backup.Games))
	}
	resetGame := backup.Games[0]
	if resetGame.LocalID != 42 {
		t.Errorf("local_id = %d, want 42", resetGame.LocalID)
	}
	if resetGame.PlaytimeResetAt != resetAt.UnixMilli() {
		t.Errorf("playtime_reset_at = %d, want %d", resetGame.PlaytimeResetAt, resetAt.UnixMilli())
	}
	if resetGame.TotalPlayTime != 90_000 {
		t.Errorf("total_play_time = %d, want 90000（只有清零之后的会话）", resetGame.TotalPlayTime)
	}
	if resetGame.LastPlayedAt != afterStart.Add(90*time.Second).UnixMilli() {
		t.Errorf("last_played_at = %d, want 清零之后那次会话的 end_time", resetGame.LastPlayedAt)
	}
	// 本地封面不迁移。
	if resetGame.CoverUri != "" {
		t.Errorf("cover_uri = %q, want empty for a local cover", resetGame.CoverUri)
	}
	if !resetGame.Hidden || !resetGame.NSFW || resetGame.PlayStatus != "playing" {
		t.Errorf("hidden/nsfw/play_status = %v/%v/%q", resetGame.Hidden, resetGame.NSFW, resetGame.PlayStatus)
	}

	favoriteGame := backup.Games[1]
	if favoriteGame.LocalID != 0 {
		t.Errorf("local_id = %d, want 0 for an unparsable legacy_local_id", favoriteGame.LocalID)
	}
	if !favoriteGame.Favorite {
		t.Error("系统收藏分类中的游戏应导出 favorite = true")
	}
	if favoriteGame.Tags != "剧情,校园" {
		t.Errorf("tags = %q, want \"剧情,校园\"", favoriteGame.Tags)
	}
	// Android 侧没有「想玩」，按契约降级为 unplayed。
	if favoriteGame.PlayStatus != "unplayed" {
		t.Errorf("play_status = %q, want unplayed", favoriteGame.PlayStatus)
	}
	if favoriteGame.CoverUri != "https://example.com/cover.jpg" {
		t.Errorf("cover_uri = %q, want the remote cover", favoriteGame.CoverUri)
	}

	if len(backup.PlaySessions) != 2 {
		t.Fatalf("导出会话数 = %d, want 2（清零之前的会话被过滤）", len(backup.PlaySessions))
	}
	exportedIDs := map[string]int64{}
	for _, session := range backup.PlaySessions {
		exportedIDs[session.SessionUUID] = session.Duration
	}
	if exportedIDs["session-after"] != 90_000 {
		t.Errorf("session-after duration = %d, want 90000 毫秒", exportedIDs["session-after"])
	}
	if exportedIDs["session-favorite"] != 60_000 {
		t.Errorf("session-favorite duration = %d, want 60000 毫秒", exportedIDs["session-favorite"])
	}
	if _, exists := exportedIDs["session-before"]; exists {
		t.Error("清零之前的会话不应导出")
	}
}

func TestYukiHubExporterExportIsIdempotentAcrossRuns(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	createdAt := time.Date(2026, time.September, 26, 10, 0, 0, 0, time.Local)
	if _, err := db.Exec(`
		INSERT INTO games (id, name, status, source_type, source_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"game-1", "重复导出的游戏", "completed", "local", "", createdAt, createdAt); err != nil {
		t.Fatalf("插入游戏失败: %v", err)
	}
	for index := 0; index < 35; index++ {
		startTime := createdAt.Add(time.Duration(index) * time.Minute)
		if _, err := db.Exec(
			`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("session-%02d", index), "game-1", startTime, startTime.Add(30*time.Second), 30, startTime); err != nil {
			t.Fatalf("插入游玩记录失败: %v", err)
		}
	}

	service := exporter.NewYukiHubExporter(context.Background(), db)
	firstPath := filepath.Join(t.TempDir(), "first.ykbak")
	secondPath := filepath.Join(t.TempDir(), "second.ykbak")
	if err := service.Export(firstPath); err != nil {
		t.Fatalf("第一次导出失败: %v", err)
	}
	if err := service.Export(secondPath); err != nil {
		t.Fatalf("第二次导出失败: %v", err)
	}

	first := readYukiHubSnapshot(t, firstPath)
	second := readYukiHubSnapshot(t, secondPath)
	if len(first.PlaySessions) != 30 || len(second.PlaySessions) != 30 {
		t.Fatalf("导出会话数 = %d/%d, want 30/30", len(first.PlaySessions), len(second.PlaySessions))
	}
	for index := range first.PlaySessions {
		if first.PlaySessions[index].SessionUUID != second.PlaySessions[index].SessionUUID {
			t.Fatalf("第 %d 条会话的 session_uuid 两次导出不一致: %q vs %q",
				index, first.PlaySessions[index].SessionUUID, second.PlaySessions[index].SessionUUID)
		}
	}
	if first.PlaySessions[0].SessionUUID != "session-05" {
		t.Errorf("最早导出的会话 = %q, want \"session-05\"（只保留最新的 30 条）", first.PlaySessions[0].SessionUUID)
	}
	if first.Games[0].TotalPlayTime != 35*30*1000 {
		t.Errorf("total_play_time = %d, want %d", first.Games[0].TotalPlayTime, 35*30*1000)
	}
}

// TestYukiHubExportThenImportRoundTrip 用真实 importer 消费导出器产出的快照，
// 验证导出/导入两个方向共用同一份契约。
//
// 重点验证 root_uri 恒空后的匹配路径：快照里的条目没有路径，Android 侧靠标题匹配
// （findByTitleForEmptyRoot），桌面端导入器同样按标题命中已有游戏；若标题无法匹配，
// 才会作为新游戏落库（此时会因「无路径」被跳过）。
func TestYukiHubExportThenImportRoundTrip(t *testing.T) {
	sourceDB, sourceCleanup := setupTestDB(t)
	defer sourceCleanup()
	// 导入目标库必须与源库分离，否则导出源本就会让导入器把每条都判为「已存在」。
	targetDB, targetCleanup := setupTestDB(t)
	defer targetCleanup()

	createdAt := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.Local)
	if _, err := sourceDB.Exec(`
		INSERT INTO games (id, name, aliases, cover_url, summary, path, game_directory, status, source_type, source_id,
			created_at, updated_at, legacy_local_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"game-1", "往返测试游戏", `["Round Trip"]`, "https://example.com/rt.jpg", "简介",
		"D:\\Games\\rt\\game.exe", "D:\\Games\\rt", "completed", "vndb", "v9000",
		createdAt, createdAt.Add(time.Hour), "42"); err != nil {
		t.Fatalf("插入游戏失败: %v", err)
	}
	if _, err := sourceDB.Exec(
		`INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"tag-1", "game-1", "剧情", "user", 1.0, false, createdAt, createdAt); err != nil {
		t.Fatalf("插入标签失败: %v", err)
	}
	startedAt := createdAt.Add(2 * time.Hour)
	if _, err := sourceDB.Exec(
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"session-round-trip", "game-1", startedAt, startedAt.Add(90*time.Second), 90, startedAt); err != nil {
		t.Fatalf("插入游玩记录失败: %v", err)
	}

	snapshotPath := filepath.Join(t.TempDir(), "round-trip.ykbak")
	if err := exporter.NewYukiHubExporter(context.Background(), sourceDB).Export(snapshotPath); err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	// 快照里不应出现任何 Windows 绝对路径。
	snapshot := readYukiHubSnapshot(t, snapshotPath)
	if len(snapshot.Games) != 1 {
		t.Fatalf("导出游戏数 = %d, want 1", len(snapshot.Games))
	}
	if snapshot.Games[0].RootUri != "" {
		t.Fatalf("root_uri = %q, want empty", snapshot.Games[0].RootUri)
	}

	// 目标库用同名同身份的游戏占位，模拟「同一款游戏已经在桌面端存在」。
	if _, err := targetDB.Exec(`
		INSERT INTO games (id, name, source_type, source_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"existing-game", "往返测试游戏", "vndb", "v9000", createdAt, createdAt); err != nil {
		t.Fatalf("插入目标库游戏失败: %v", err)
	}

	imported, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).Import(snapshotPath, false, importer.SamePathActionSkip)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if imported.Skipped != 1 || imported.Success != 0 {
		t.Fatalf("导入结果 = success %d / skipped %d (%v), want 0/1：同标题已存在应被跳过",
			imported.Success, imported.Skipped, imported.SkippedNames)
	}

	// 导入方向对称性：对端没有路径，桌面端不应凭空写入 path。
	var count int
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM games WHERE id = ? AND COALESCE(path, '') = ''`, "existing-game").Scan(&count); err != nil {
		t.Fatalf("查询目标库游戏失败: %v", err)
	}
	if count != 1 {
		t.Error("导入不应把对端路径写进桌面端 path")
	}
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM games WHERE id = 'existing-game'`).Scan(&count); err != nil {
		t.Fatalf("查询目标库游戏失败: %v", err)
	}
	if count != 1 {
		t.Errorf("导入后同标题不应产生重复条目, got %d", count)
	}
}

// TestYukiHubImportAppliesFavoriteToSystemCategory 验证导入方向把快照的 favorite
// 落到「收藏」系统分类。
//
// 导出方向早已读取该分类（loadFavorites），导入方向此前完全丢弃 favorite，
// 造成「桌面端导出 → 手机版 → 回导」丢失收藏的不对称。
func TestYukiHubImportAppliesFavoriteToSystemCategory(t *testing.T) {
	targetDB, cleanup := setupTestDB(t)
	defer cleanup()

	createdAt := time.Date(2026, time.September, 28, 9, 0, 0, 0, time.Local)
	backup := yukihub.Backup{
		App:       "YukiHub",
		Schema:    5,
		CreatedAt: createdAt.UnixMilli(),
		Games: []yukihub.Game{
			{LocalID: 1, Title: "收藏的游戏", Favorite: true, CreatedAt: createdAt.UnixMilli(), UpdatedAt: createdAt.UnixMilli()},
			{LocalID: 2, Title: "普通的游戏", CreatedAt: createdAt.UnixMilli(), UpdatedAt: createdAt.UnixMilli()},
		},
	}
	snapshotPath := writeSnapshotFile(t, t.TempDir(), backup)

	result, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).
		Import(snapshotPath, false, importer.SamePathActionSkip)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if result.Success != 2 || result.Failed != 0 {
		t.Fatalf("导入结果 = success %d / failed %d, want 2/0", result.Success, result.Failed)
	}

	var favoriteID string
	if err := targetDB.QueryRow(`SELECT id FROM games WHERE name = ?`, "收藏的游戏").Scan(&favoriteID); err != nil {
		t.Fatalf("查询收藏游戏失败: %v", err)
	}
	var count int
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM game_categories WHERE game_id = ? AND category_id = ?`,
		favoriteID, gamehelper.SystemFavoritesCategoryID).Scan(&count); err != nil {
		t.Fatalf("查询收藏分类失败: %v", err)
	}
	if count != 1 {
		t.Errorf("收藏游戏在 system:favorites 中的记录数 = %d, want 1", count)
	}

	var plainID string
	if err := targetDB.QueryRow(`SELECT id FROM games WHERE name = ?`, "普通的游戏").Scan(&plainID); err != nil {
		t.Fatalf("查询普通游戏失败: %v", err)
	}
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM game_categories WHERE game_id = ?`, plainID).Scan(&count); err != nil {
		t.Fatalf("查询普通游戏分类失败: %v", err)
	}
	if count != 0 {
		t.Errorf("非收藏游戏不应产生分类记录, got %d", count)
	}
}

// TestYukiHubMetadataCacheRoundTrip 验证元数据缓存负载在两个方向都能原样往返。
//
// 往返语义：桌面端存量缓存（已是 Android VnMetadata 结构）→ 导出快照 → 导入另一个库
// → 再导出，JSON 必须逐字节一致，且游戏仍按 local_id 关联。桌面端自建条目没有
// legacy_local_id，无从映射回 Android 的整数 local_id，导出时必须跳过，否则会产出孤儿缓存。
func TestYukiHubMetadataCacheRoundTrip(t *testing.T) {
	sourceDB, sourceCleanup := setupTestDB(t)
	defer sourceCleanup()
	targetDB, targetCleanup := setupTestDB(t)
	defer targetCleanup()

	payload := `{"id":"v9000","chineseTitle":"往返缓存游戏","romanTitle":"Round Trip Cache","screenshotUrls":["https://example.com/s1.jpg"]}`
	createdAt := time.Date(2026, time.September, 28, 11, 0, 0, 0, time.Local)
	if _, err := sourceDB.Exec(`
		INSERT INTO games (id, name, status, source_type, source_id, created_at, updated_at, legacy_local_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"game-cache", "往返缓存游戏", "completed", "vndb", "v9000", createdAt, createdAt.Add(time.Hour), "42"); err != nil {
		t.Fatalf("插入游戏失败: %v", err)
	}
	if _, err := sourceDB.Exec(`
		INSERT INTO games (id, name, status, source_type, source_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"game-orphan", "自建条目", "not_started", "vndb", "v1", createdAt, createdAt); err != nil {
		t.Fatalf("插入自建游戏失败: %v", err)
	}
	for _, entry := range []struct {
		gameID   string
		sourceID string
		payload  string
	}{
		{"game-cache", "v9000", payload},
		{"game-orphan", "v1", `{"id":"v1"}`},
	} {
		if _, err := sourceDB.Exec(`
			INSERT INTO game_metadata_sources (game_id, source_type, source_id, cache_json, cached_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			entry.gameID, "vndb", entry.sourceID, entry.payload, createdAt, createdAt, createdAt); err != nil {
			t.Fatalf("插入元数据缓存失败: %v", err)
		}
	}

	snapshotPath := filepath.Join(t.TempDir(), "metadata-cache.ykbak")
	if err := exporter.NewYukiHubExporter(context.Background(), sourceDB).Export(snapshotPath); err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	snapshot := readYukiHubSnapshot(t, snapshotPath)
	if len(snapshot.MetadataCache) != 1 {
		t.Fatalf("导出缓存条目数 = %d, want 1（无 legacy_local_id 的条目应被跳过）", len(snapshot.MetadataCache))
	}
	entry := snapshot.MetadataCache[0]
	if entry.GameLocalID != 42 || entry.Source != "vndb" || entry.SourceID != "v9000" {
		t.Fatalf("缓存条目身份 = %+v, want local_id 42 / vndb / v9000", entry)
	}
	if entry.JSON != payload {
		t.Fatalf("导出缓存负载 = %q, want %q", entry.JSON, payload)
	}

	imported, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).
		Import(snapshotPath, false, importer.SamePathActionSkip)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	// 两条游戏都会作为新条目落库（自建条目只是没有缓存，游戏本身仍会导出与导入）。
	if imported.Success != 2 || imported.Failed != 0 {
		t.Fatalf("导入结果 = success %d / failed %d (%v), want 2/0", imported.Success, imported.Failed, imported.FailedNames)
	}

	var persisted string
	if err := targetDB.QueryRow(`
		SELECT s.cache_json
		FROM game_metadata_sources s
		JOIN games g ON g.id = s.game_id
		WHERE g.legacy_local_id = '42' AND s.source_type = 'vndb'`).Scan(&persisted); err != nil {
		t.Fatalf("查询导入后的缓存失败: %v", err)
	}
	if persisted != payload {
		t.Fatalf("导入后的缓存负载 = %q, want %q", persisted, payload)
	}

	reExportedPath := filepath.Join(t.TempDir(), "metadata-cache-again.ykbak")
	if err := exporter.NewYukiHubExporter(context.Background(), targetDB).Export(reExportedPath); err != nil {
		t.Fatalf("再次导出失败: %v", err)
	}
	again := readYukiHubSnapshot(t, reExportedPath)
	if len(again.MetadataCache) != 1 {
		t.Fatalf("再次导出缓存条目数 = %d, want 1", len(again.MetadataCache))
	}
	if again.MetadataCache[0].JSON != payload || again.MetadataCache[0].GameLocalID != 42 {
		t.Fatalf("往返后的缓存 = %+v, want 与首次导出逐字节一致", again.MetadataCache[0])
	}
}

// newTestImporterDependencies 构造最小可用的导入依赖：真实 Committer + 直写 play_sessions，
// 与 ImportService.importerDependencies 生产装配保持一致（仅裁掉封面下载与 Wails 相关部分）。
func newTestImporterDependencies(db *sql.DB) importer.Dependencies {
	ctx := context.Background()
	committer := importer.NewCommitter(importer.CommitDependencies{Ctx: ctx, DB: db})
	return importer.Dependencies{
		Ctx:       ctx,
		ListGames: committer.ListGames,
		AddItems:  committer.AddItems,
		AddSessions: func(sessions []models.PlaySession) error {
			for _, session := range sessions {
				if _, err := db.Exec(
					`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					session.ID, session.GameID, session.StartTime, session.EndTime, session.Duration, session.UpdatedAt); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func readYukiHubSnapshot(t *testing.T, path string) yukihub.Backup {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开快照失败: %v", err)
	}
	defer file.Close()

	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("解压快照失败: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("读取快照失败: %v", err)
	}
	var backup yukihub.Backup
	if err := json.Unmarshal(data, &backup); err != nil {
		t.Fatalf("解析快照失败: %v", err)
	}
	return backup
}
