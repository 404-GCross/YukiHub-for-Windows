package test

import (
	"context"
	"path/filepath"
	"testing"
	"time"
	"yukihub/internal/service/exporter"
	"yukihub/internal/service/importer"
)

// TestYukiHubReimportWithSkipActionIsIdempotent 默认的 skip 动作下，同一份快照
// 导入两次不产生重复：第二次整个条目被「已存在」跳过（YukiHub 条目无路径，
// name+空路径即视为同一条目），会话数保持不变。
//
// 合并路径的语义由 TestYukiHubImportMergeSessionsTakesMaxPlaytime 覆盖。
func TestYukiHubReimportWithSkipActionIsIdempotent(t *testing.T) {
	sourceDB, sourceCleanup := setupTestDB(t)
	defer sourceCleanup()
	targetDB, targetCleanup := setupTestDB(t)
	defer targetCleanup()

	createdAt := time.Date(2026, time.September, 28, 10, 0, 0, 0, time.Local)
	if _, err := sourceDB.Exec(`
		INSERT INTO games (id, name, status, source_type, source_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"game-1", "重复导入探针", "playing", "local", "", createdAt, createdAt); err != nil {
		t.Fatalf("插入游戏失败: %v", err)
	}
	startedAt := createdAt.Add(time.Hour)
	if _, err := sourceDB.Exec(
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"11111111-2222-3333-4444-555555555555", "game-1",
		startedAt, startedAt.Add(90*time.Second), 90, startedAt); err != nil {
		t.Fatalf("插入游玩记录失败: %v", err)
	}

	snapshotPath := filepath.Join(t.TempDir(), "reimport.ykbak")
	if err := exporter.NewYukiHubExporter(context.Background(), sourceDB).Export(snapshotPath); err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	service := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB))
	for round := 1; round <= 2; round++ {
		result, err := service.Import(snapshotPath, false, importer.SamePathActionSkip)
		if err != nil {
			t.Fatalf("第 %d 次导入失败: %v", round, err)
		}
		t.Logf("第 %d 次导入: success=%d skipped=%d failed=%d sessions=%d skippedNames=%v",
			round, result.Success, result.Skipped, result.Failed, result.SessionsImported, result.SkippedNames)
	}

	var sessionCount int
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM play_sessions`).Scan(&sessionCount); err != nil {
		t.Fatalf("统计会话失败: %v", err)
	}
	var gameCount int
	if err := targetDB.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&gameCount); err != nil {
		t.Fatalf("统计游戏失败: %v", err)
	}
	t.Logf("导入两次后的会话数 = %d（期望 1），游戏数 = %d（期望 1）", sessionCount, gameCount)

	if sessionCount != 1 {
		t.Errorf("会话重复落库：session_uuid 幂等未生效，got %d want 1", sessionCount)
	}
}
