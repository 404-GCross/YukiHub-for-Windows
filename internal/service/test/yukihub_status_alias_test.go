package test

import (
	"strings"
	"testing"
	"time"

	"yukihub/internal/models/yukihub"
	"yukihub/internal/service/importer"

	_ "github.com/duckdb/duckdb-go/v2"
)

// TestYukiHubImportPersistsStatusAndAliases 验证导入方向把 play_status 与
// 原文名（aliases）真正写进库。
//
// 这两个字段此前只在前半段被映射、却不在落库的列清单里：
//   - status 全部拿到列默认值「未玩」，手机版的 completed / playing / dropped 全丢
//   - aliases 完全没有落库，原文名与罗马字标题丢失
//
// 用手机版真实备份（32 个游戏、4 种状态）才暴露出来，这里用合成快照锁死。
func TestYukiHubImportPersistsStatusAndAliases(t *testing.T) {
	targetDB, cleanup := setupTestDB(t)
	defer cleanup()

	createdAt := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.Local)
	updatedAt := createdAt.Add(time.Hour)

	newGame := func(localID int64, title, originalTitle, status string) yukihub.Game {
		return yukihub.Game{
			LocalID:         localID,
			Title:           title,
			OriginalTitle:   originalTitle,
			PlayStatus:      status,
			TotalPlayTime:   0,
			CreatedAt:       createdAt.UnixMilli(),
			UpdatedAt:       updatedAt.UnixMilli(),
			PlaytimeResetAt: 0,
		}
	}

	backup := yukihub.Backup{
		App:       "YukiHub",
		Schema:    5,
		CreatedAt: updatedAt.UnixMilli(),
		Games: []yukihub.Game{
			newGame(1, "已通关的游戏", "クリア済みゲーム", "completed"),
			newGame(2, "在玩的游戏", "Playing Game", "playing"),
			newGame(3, "抛弃的游戏", "Dropped Game", "dropped"),
			newGame(4, "搁置的游戏", "OnHold Game", "onhold"),
			// 手机版 normalizePlayStatus 只产出四态 + onhold；空值应回落「未玩」，
			// 而不是写进一个空字符串让前端画不出徽标。
			newGame(5, "状态为空的游戏", "", ""),
		},
	}
	snapshotPath := writeSnapshotFile(t, t.TempDir(), backup)

	result, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).
		Import(snapshotPath, false, importer.SamePathActionSkip)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if result.Success != 5 || result.Failed != 0 {
		t.Fatalf("导入结果 = success %d / failed %d, want 5/0", result.Success, result.Failed)
	}

	expectations := []struct {
		name   string
		status string
		alias  string
	}{
		{"已通关的游戏", "completed", "クリア済みゲーム"},
		{"在玩的游戏", "playing", "Playing Game"},
		{"抛弃的游戏", "dropped", "Dropped Game"},
		{"搁置的游戏", "onhold", "OnHold Game"},
		{"状态为空的游戏", "unplayed", ""},
	}

	for _, want := range expectations {
		var status string
		var aliases string
		if err := targetDB.QueryRow(
			`SELECT COALESCE(status, ''), COALESCE(aliases, '[]') FROM games WHERE name = ?`,
			want.name,
		).Scan(&status, &aliases); err != nil {
			t.Fatalf("查询 %s 失败: %v", want.name, err)
		}
		if status != want.status {
			t.Errorf("%s 的 status = %q, want %q", want.name, status, want.status)
		}
		if want.alias != "" && !strings.Contains(aliases, want.alias) {
			t.Errorf("%s 的 aliases = %q, 缺少原文名 %q", want.name, aliases, want.alias)
		}
	}

	// 状态必须是五种取值之一，不能出现空串（前端按状态画徽标）。
	if blanks := countRows(t, targetDB, `SELECT COUNT(*) FROM games WHERE TRIM(COALESCE(status, '')) = ''`); blanks != 0 {
		t.Errorf("有 %d 条游戏 status 为空", blanks)
	}
}
