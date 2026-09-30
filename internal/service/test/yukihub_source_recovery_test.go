package test

import (
	"testing"
	"time"

	"yukihub/internal/models/yukihub"
	"yukihub/internal/service/importer"
)

// TestYukiHubImportRestoresNextMoeIdentity 锁定「nextmoe 来源丢失」的整条修复链路。
//
// 背景（用户实测报的现象）：手机版 `settings.metadata_source = "nextmoe"`，
// 但导入器的来源映射表与兜底优先级名单里都漏了 nextmoe，于是这些游戏被归到
// 名单里靠前的 vndb 上。用户库里的错误记录**要靠重新导入一次来纠正**——
// 合并路径（updateImportedItemMetadata）会无条件覆盖 source_type / source_id，
// 所以「按标题匹配到的旧行」会被改成 nextmoe。这个测试同时守住这两点：
//
//  1. 导入时按手机端的偏好来源选中 nextmoe 条目；
//  2. 已经落成 vndb 的旧行，重新导入后会被纠正成 nextmoe。
func TestYukiHubImportRestoresNextMoeIdentity(t *testing.T) {
	targetDB, targetCleanup := setupTestDB(t)
	defer targetCleanup()

	createdAt := time.Date(2026, time.September, 30, 10, 0, 0, 0, time.Local)

	// 先造一条「上一次导入留下的」错误记录：同一款游戏被标成了 vndb。
	// root_uri 为空（YukiHub 导出的语义），对端会退回按标题匹配。
	const gameTitle = "天使☆嚣嚣 RE-BOOT!"
	if _, err := targetDB.Exec(`
		INSERT INTO games (id, name, summary, status, source_type, source_id, created_at, updated_at, path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"existing-nextmoe", gameTitle, "旧简介", "playing", "vndb", "v40520",
		createdAt, createdAt, ""); err != nil {
		t.Fatalf("插入旧记录失败: %v", err)
	}

	backup := yukihub.Backup{
		App:       "YukiHub",
		Schema:    5,
		CreatedAt: createdAt.Add(time.Hour).UnixMilli(),
		Settings:  yukihub.BackupSettings{MetadataSource: "nextmoe"},
		Games: []yukihub.Game{
			{
				LocalID:    1,
				Title:      gameTitle,
				PlayStatus: "playing",
				CreatedAt:  createdAt.UnixMilli(),
				UpdatedAt:  createdAt.Add(time.Hour).UnixMilli(),
			},
		},
		MetadataCache: []yukihub.MetadataCache{
			{
				GameLocalID: 1,
				Source:      "vndb",
				SourceID:    "v40520",
				JSON:        `{"id":"v40520","chineseTitle":"vndb 侧标题","coverUrl":"https://t.vndb.org/cv.t/66/88666.jpg"}`,
				UpdatedAt:   createdAt.UnixMilli(),
			},
			{
				GameLocalID: 1,
				Source:      "nextmoe",
				SourceID:    "23",
				JSON:        `{"id":"23","chineseTitle":"未萌目录侧标题","coverUrl":"https://image.kungal.iloveren.link/x.jpg"}`,
				UpdatedAt:   createdAt.UnixMilli(),
			},
		},
	}

	snapshotPath := writeSnapshotFile(t, t.TempDir(), backup)
	result, err := importer.NewYukiHubImporter(newTestImporterDependencies(targetDB)).
		Import(snapshotPath, false, importer.SamePathActionMerge)
	if err != nil {
		t.Fatalf("合并导入失败: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("导入结果 = success %d / skipped %d (%v), want 1/0",
			result.Success, result.Skipped, result.SkippedNames)
	}

	var sourceType, sourceID, name string
	if err := targetDB.QueryRow(
		`SELECT COALESCE(source_type, ''), COALESCE(source_id, ''), name
		 FROM games WHERE name = ?`, gameTitle).
		Scan(&sourceType, &sourceID, &name); err != nil {
		t.Fatalf("查询游戏失败: %v", err)
	}

	if sourceType != "nextmoe" {
		t.Errorf("重新导入后 source_type = %q, want nextmoe（旧的 vndb 记录应被纠正）", sourceType)
	}
	// source_id 必须是 nextmoe 那个（23），而不是 vndb 的 v40520——这才说明
	// 「偏好来源」判定真的生效了，身份不是靠兜底名单碰出来的。
	if sourceID != "23" {
		t.Errorf("重新导入后 source_id = %q, want 23", sourceID)
	}
	// name 取的是备份里的游戏标题（Android 侧的游戏名），不是元数据标题
	if name != gameTitle {
		t.Errorf("name = %q, want %q", name, gameTitle)
	}

	if count := countRows(t, targetDB, `SELECT COUNT(*) FROM games`); count != 1 {
		t.Errorf("games 行数 = %d, want 1（应合并而不是新增）", count)
	}
}
