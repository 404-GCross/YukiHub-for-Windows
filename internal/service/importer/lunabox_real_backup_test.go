package importer

import (
	"os"
	"testing"
)

// 用真实的 LunaBox 备份核对解析结果。
//
// 只有在设置了 YUKIHUB_REAL_LUNABOX_BACKUP（指向一个真实 .zip 备份）时才执行，
// 否则跳过 —— 与 real_backup_probe_test.go 同样的做法，避免 CI 依赖本机文件。
//
// 这条用例的价值在于：造出来的 CSV 永远「刚好符合我的假设」，
// 而真实备份才会暴露列名差异、时间格式差异、封面引用格式差异。
func TestReadLunaBoxArchiveRealBackup(t *testing.T) {
	zipPath := os.Getenv("YUKIHUB_REAL_LUNABOX_BACKUP")
	if zipPath == "" {
		t.Skip("未设置 YUKIHUB_REAL_LUNABOX_BACKUP，跳过真实备份核对")
	}

	archive, err := readLunaBoxArchive(zipPath)
	if err != nil {
		t.Fatalf("解析真实备份失败: %v", err)
	}
	if len(archive.entries) == 0 {
		t.Fatalf("真实备份里没有解析出任何游戏")
	}

	localCovers, missingCovers, remoteCovers := 0, 0, 0
	for _, entry := range archive.entries {
		game := entry.game
		if game.Name == "" {
			t.Errorf("存在没有名称的条目: %+v", game)
		}
		if game.ID == "" {
			t.Errorf("%s 没有生成主键", game.Name)
		}
		if game.CreatedAt.IsZero() {
			t.Errorf("%s 的 created_at 未解析", game.Name)
		}
		// 来源必须是已知来源之一（localhost 除外时说明映射漏项了）
		switch game.SourceType {
		case "vndb", "bangumi", "ymgal", "hikarinagi", "nextmoe",
			"steam", "dlsite", "touchgal", "erogamescape", "local":
		default:
			t.Errorf("%s 的来源 %q 不在已知集合里（映射可能漏项）", game.Name, game.SourceType)
		}

		switch {
		case entry.coverFile != "":
			localCovers++
			if len(archive.covers[entry.coverFile]) == 0 {
				missingCovers++
				t.Errorf("%s 引用了本地封面 %s，但 ZIP 里没有该文件", game.Name, entry.coverFile)
			}
		case game.CoverURL != "":
			remoteCovers++
		}

		t.Logf("%-28s 状态=%-9s 来源=%-10s 路径=%v 会话=%d 标签=%d",
			truncateForLog(game.Name), game.Status, game.SourceType,
			game.Path != "", len(archive.sessions[entry.lunaBoxID]), len(entry.tags))
	}

	// 游玩记录必须能通过 LunaBox 的 id 关联到游戏（本机主键是新生成的 uuid，
	// 两者不能混用），否则导入后时长全丢。
	matchedSessions := 0
	for _, entry := range archive.entries {
		sessions := rekeyLunaBoxSessions(archive.sessions[entry.lunaBoxID], entry.game.ID)
		for _, session := range sessions {
			if session.GameID != entry.game.ID {
				t.Errorf("%s 的游玩记录没有换成本机主键: %q", entry.game.Name, session.GameID)
			}
		}
		matchedSessions += len(sessions)
	}
	allSessions := 0
	for _, sessions := range archive.sessions {
		allSessions += len(sessions)
	}
	if matchedSessions != allSessions {
		t.Errorf("可按游戏关联的游玩记录 = %d, 备份里总共有 %d（存在孤立记录）",
			matchedSessions, allSessions)
	}
	t.Logf("汇总: 游戏=%d 游玩记录=%d 封面(本地/远程)=%d/%d 本地封面缺失=%d",
		len(archive.entries), allSessions, localCovers, remoteCovers, missingCovers)
}

func truncateForLog(name string) string {
	runes := []rune(name)
	if len(runes) <= 26 {
		return name
	}
	return string(runes[:26]) + "…"
}
