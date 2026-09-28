package test

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
	"yukihub/internal/models/yukihub"
	"yukihub/internal/service/importer"
)

// 合并语义验证：契约要求「总时长取最大值，不做覆盖」。
//
// 桌面端没有游戏级 total_play_time 列，总时长由会话聚合而来，因此"取最大值"
// 通过两个机制组合实现：
//  1. 会话并集去重（按 game_id + start + end），两端相同的物理会话只留一份；
//  2. 聚合补偿（convertYukiHubSessions）：快照 total_play_time 超过已录会话
//     总和的差额，补成一条确定性 UUID 的聚合会话。
//
// 最终桌面总时长 = max(桌面已录时长, 快照 total_play_time)。

func buildMergeSnapshot(t *testing.T, dir string) string {
	t.Helper()

	createdAt := time.Date(2026, time.September, 28, 8, 0, 0, 0, time.Local)
	s10 := time.Date(2026, time.September, 28, 10, 0, 0, 0, time.Local)
	s11 := time.Date(2026, time.September, 28, 11, 0, 0, 0, time.Local)

	// 快照里的会话与桌面端已有的那条是同一次物理游玩（同 UUID 同时间）。
	backup := yukihub.Backup{
		App:       "YukiHub",
		Schema:    5,
		CreatedAt: createdAt.UnixMilli(),
		Games: []yukihub.Game{{
			LocalID:       7,
			Title:         "合并目标",
			PlayStatus:    "playing",
			TotalPlayTime: 150_000, // 150 秒：桌面端只有 60 秒，快照更长
			CreatedAt:     createdAt.UnixMilli(),
			UpdatedAt:     createdAt.UnixMilli(),
		}},
		PlaySessions: []yukihub.PlaySession{
			{SessionUUID: "aaaaaaaa-1111-2222-3333-444444444444", GameLocalID: 7, GameTitle: "合并目标",
				StartTime: s10.UnixMilli(), EndTime: s10.Add(60 * time.Second).UnixMilli(),
				Duration: 60_000, CreatedAt: s10.UnixMilli(), UpdatedAt: s10.UnixMilli()},
			{SessionUUID: "bbbbbbbb-1111-2222-3333-444444444444", GameLocalID: 7, GameTitle: "合并目标",
				StartTime: s11.UnixMilli(), EndTime: s11.Add(30 * time.Second).UnixMilli(),
				Duration: 30_000, CreatedAt: s11.UnixMilli(), UpdatedAt: s11.UnixMilli()},
		},
	}
	return writeSnapshotFile(t, dir, backup)
}

func writeSnapshotFile(t *testing.T, dir string, backup yukihub.Backup) string {
	t.Helper()

	// 直接序列化为 gzip 文件，绕过导出器（导出器只读 DuckDB），
	// 便于构造导入侧的边界用例。
	data, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("序列化快照失败: %v", err)
	}
	path := filepath.Join(dir, "merge.ykbak")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建快照文件失败: %v", err)
	}
	writer := gzip.NewWriter(file)
	if _, err := writer.Write(data); err != nil {
		file.Close()
		t.Fatalf("写入快照失败: %v", err)
	}
	if err := writer.Close(); err != nil {
		file.Close()
		t.Fatalf("关闭快照失败: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("保存快照失败: %v", err)
	}
	return path
}

// TestYukiHubImportMergeSessionsTakesMaxPlaytime 验证 merge_sessions 动作下
// 两端时长合并结果为取最大值。
func TestYukiHubImportMergeSessionsTakesMaxPlaytime(t *testing.T) {
	targetDB, targetCleanup := setupTestDB(t)
	defer targetCleanup()

	// 桌面端已有同名游戏，已录 60 秒会话（与快照中的第一条是同一次物理游玩）。
	createdAt := time.Date(2026, time.September, 28, 8, 0, 0, 0, time.Local)
	s10 := time.Date(2026, time.September, 28, 10, 0, 0, 0, time.Local)
	if _, err := targetDB.Exec(`
		INSERT INTO games (id, name, summary, status, source_type, source_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"existing-1", "合并目标", "桌面端简介", "playing", "local", "", createdAt, createdAt); err != nil {
		t.Fatalf("插入已有游戏失败: %v", err)
	}
	if _, err := targetDB.Exec(
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"aaaaaaaa-1111-2222-3333-444444444444", "existing-1",
		s10, s10.Add(60*time.Second), 60, s10); err != nil {
		t.Fatalf("插入已有会话失败: %v", err)
	}

	snapshotPath := buildMergeSnapshot(t, t.TempDir())
	result, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).
		Import(snapshotPath, false, importer.SamePathActionMergeSessions)
	if err != nil {
		t.Fatalf("合并导入失败: %v", err)
	}
	if result.Success != 1 || result.Skipped != 0 {
		t.Fatalf("合并导入结果 = success %d / skipped %d (%v), want 1/0",
			result.Success, result.Skipped, result.SkippedNames)
	}

	// 会话并集：60s（去重保留）+ 30s（新增）+ 60s（聚合补偿 150-90）= 150 秒。
	var totalSeconds int
	if err := targetDB.QueryRow(
		`SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE game_id = 'existing-1'`).Scan(&totalSeconds); err != nil {
		t.Fatalf("统计时长失败: %v", err)
	}
	if totalSeconds != 150 {
		t.Errorf("合并后总时长 = %d 秒, want 150（取最大值）", totalSeconds)
	}
	var sessionCount int
	if err := targetDB.QueryRow(
		`SELECT COUNT(*) FROM play_sessions WHERE game_id = 'existing-1'`).Scan(&sessionCount); err != nil {
		t.Fatalf("统计会话失败: %v", err)
	}
	if sessionCount != 3 {
		t.Errorf("合并后会话数 = %d, want 3（并集去重 + 聚合补偿）", sessionCount)
	}
}

// TestYukiHubImportMergeUpdatesMetadata 验证 merge 动作下元数据被 Android 侧
// 权威数据覆盖，但桌面端本机字段（path）不受影响。
func TestYukiHubImportMergeUpdatesMetadata(t *testing.T) {
	targetDB, targetCleanup := setupTestDB(t)
	defer targetCleanup()

	createdAt := time.Date(2026, time.September, 28, 8, 0, 0, 0, time.Local)
	if _, err := targetDB.Exec(`
		INSERT INTO games (id, name, summary, status, source_type, source_id, created_at, updated_at, path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"existing-1", "合并目标", "桌面端旧简介", "completed", "local", "", createdAt, createdAt,
		"D:\\Games\\local\\game.exe"); err != nil {
		t.Fatalf("插入已有游戏失败: %v", err)
	}

	backup := yukihub.Backup{
		App:       "YukiHub",
		Schema:    5,
		CreatedAt: createdAt.UnixMilli(),
		Games: []yukihub.Game{{
			LocalID:       7,
			Title:         "合并目标",
			Description:   "Android 侧新简介",
			PlayStatus:    "playing",
			TotalPlayTime: 0,
			CreatedAt:     createdAt.UnixMilli(),
			UpdatedAt:     createdAt.UnixMilli(),
		}},
	}
	snapshotPath := writeSnapshotFile(t, t.TempDir(), backup)

	result, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).
		Import(snapshotPath, false, importer.SamePathActionMerge)
	if err != nil {
		t.Fatalf("合并导入失败: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("合并导入结果 = success %d / skipped %d (%v), want 1/0",
			result.Success, result.Skipped, result.SkippedNames)
	}

	var summary, gamePath, status string
	if err := targetDB.QueryRow(
		`SELECT summary, COALESCE(path, ''), status FROM games WHERE id = 'existing-1'`).
		Scan(&summary, &gamePath, &status); err != nil {
		t.Fatalf("查询合并结果失败: %v", err)
	}
	if summary != "Android 侧新简介" {
		t.Errorf("summary = %q, want Android 侧新简介（Android 为权威源）", summary)
	}
	if gamePath != `D:\Games\local\game.exe` {
		t.Errorf("path = %q, want 保留桌面端本机路径", gamePath)
	}
	// 既有行为：update 路径（updateImportedItemMetadata）不含 status 列，
	// 游玩状态不会被导入数据覆盖。
	if status != "completed" {
		t.Errorf("status = %q, want completed（update 路径不覆盖游玩状态）", status)
	}
}
