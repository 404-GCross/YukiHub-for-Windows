package test

import (
	"database/sql"
	"os"
	"testing"

	"yukihub/internal/service/importer"
)

// TestYukiHubImportRealBackup 用真实手机版导出的 .ykbak 做一次端到端导入核对。
//
// 默认跳过（不会进 CI）；需要时显式指一个文件：
//
//	YUKIHUB_REAL_BACKUP=D:/path/yukihub_backup_xxx.ykbak go test ./internal/service/test/ \
//	  -run TestYukiHubImportRealBackup -v
func TestYukiHubImportRealBackup(t *testing.T) {
	backupPath := os.Getenv("YUKIHUB_REAL_BACKUP")
	if backupPath == "" {
		t.Skip("未设置 YUKIHUB_REAL_BACKUP，跳过真实备份导入核对")
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("备份文件不可读: %v", err)
	}

	targetDB, cleanup := setupTestDB(t)
	defer cleanup()

	result, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).
		Import(backupPath, false, importer.SamePathActionMerge)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}

	games := countRows(t, targetDB, `SELECT COUNT(*) FROM games`)
	sessions := countRows(t, targetDB, `SELECT COUNT(*) FROM play_sessions`)
	// 收藏不是 games 的列，而是系统分类 system:favorites
	favorites := countRows(t, targetDB, `SELECT COUNT(*) FROM game_categories gc JOIN categories c ON c.id = gc.category_id WHERE c.is_system = TRUE`)
	hidden := countRows(t, targetDB, `SELECT COUNT(*) FROM games WHERE COALESCE(hidden, FALSE)`)
	nsfw := countRows(t, targetDB, `SELECT COUNT(*) FROM games WHERE COALESCE(is_nsfw, FALSE)`)
	resets := countRows(t, targetDB, `SELECT COUNT(*) FROM games WHERE playtime_reset_at IS NOT NULL`)
	statuses := countRows(t, targetDB, `SELECT COUNT(DISTINCT status) FROM games`)
	sources := countRows(t, targetDB, `SELECT COUNT(*) FROM game_metadata_sources`)
	tags := countRows(t, targetDB, `SELECT COUNT(*) FROM game_tags`)

	t.Logf("导入结果: success=%d failed=%d skipped=%d sessions=%d",
		result.Success, result.Failed, result.Skipped, result.SessionsImported)
	t.Logf("落库: games=%d sessions=%d 收藏=%d 隐藏=%d NSFW=%d 清零=%d 状态种类=%d 元数据来源=%d 标签=%d",
		games, sessions, favorites, hidden, nsfw, resets, statuses, sources, tags)

	// 状态分布：手机端 4 种状态（completed 21 / unplayed 9 / playing 1 / dropped 1）
	dumpCounts(t, targetDB, "状态分布", `SELECT COALESCE(status, '<null>'), COUNT(*) FROM games GROUP BY 1 ORDER BY 2 DESC`)
	dumpCounts(t, targetDB, "分类行", `SELECT id, COUNT(*) FROM categories GROUP BY 1`)
	dumpCounts(t, targetDB, "game_categories 全部", `SELECT category_id, COUNT(*) FROM game_categories GROUP BY 1`)
	dumpCounts(t, targetDB, "别名情况", `SELECT CASE WHEN COALESCE(aliases, '[]') IN ('', '[]') THEN '空' ELSE '有别名' END, COUNT(*) FROM games GROUP BY 1`)
	dumpCounts(t, targetDB, "每次游玩记录类型", `SELECT CASE WHEN id LIKE 'agg%' OR id LIKE '%aggregate%' THEN '聚合补偿' ELSE '普通' END, COUNT(*) FROM play_sessions GROUP BY 1`)
	dumpCounts(t, targetDB, "元数据来源分布", `SELECT source_type, COUNT(*) FROM game_metadata_sources GROUP BY 1 ORDER BY 2 DESC`)

	// 时长单位核对：库里是秒（games 没有 total_play_time 列，聚合在 play_sessions）
	rows, err := targetDB.Query(`
		SELECT g.name,
		       COALESCE((SELECT SUM(ps.duration) FROM play_sessions ps WHERE ps.game_id = g.id), 0) AS session_sum,
		       g.status
		FROM games g
		WHERE COALESCE((SELECT SUM(ps.duration) FROM play_sessions ps WHERE ps.game_id = g.id), 0) > 0
		ORDER BY session_sum DESC
		LIMIT 5`)
	if err != nil {
		t.Fatalf("查询时长失败: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, status string
		var sum int64
		if err := rows.Scan(&name, &sum, &status); err != nil {
			t.Fatalf("扫描时长失败: %v", err)
		}
		t.Logf("  时长 Top: %s | 秒=%d | status=%s", name, sum, status)
	}

	if blanks := countRows(t, targetDB, `SELECT COUNT(*) FROM games WHERE TRIM(COALESCE(name, '')) = ''`); blanks != 0 {
		t.Errorf("库里有 %d 条空名游戏，违反「无标题跳过」契约", blanks)
	}
}

func dumpCounts(t *testing.T, db *sql.DB, label string, query string) {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Logf("  %s: 查询失败 %v", label, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			t.Logf("  %s: 扫描失败 %v", label, err)
			return
		}
		t.Logf("  %s: %s = %d", label, key, count)
	}
}
