package importer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yukihub/internal/common/enums"
	"yukihub/internal/models"
)

// LunaBox 的游玩状态必须显式换算成 YukiHub 的五态。
//
// 不换算不会报错，只会静默归成「未玩」——
// 表现为「搁置 / 在玩 / 已通关的游戏导入后全都变成未玩」，
// 与之前 nextmoe 来源被静默归成 local 是同一类问题。
func TestMapLunaBoxStatusCoversUpstreamValues(t *testing.T) {
	cases := map[string]enums.GameStatus{
		// LunaBox 独有的两个「未开始」态都并入 unplayed（手机版没有「想玩」）
		"not_started":  enums.StatusUnplayed,
		"want_to_play": enums.StatusUnplayed,
		"unplayed":     enums.StatusUnplayed,
		"":             enums.StatusUnplayed,
		"unknown_xxx":  enums.StatusUnplayed,

		"playing":     enums.StatusPlaying,
		"in_progress": enums.StatusPlaying,

		"completed": enums.StatusCompleted,
		"finished":  enums.StatusCompleted,

		// on_hold → onhold 是纯写法差异，最容易漏掉
		"on_hold": enums.StatusOnHold,
		"onhold":  enums.StatusOnHold,

		"dropped":   enums.StatusDropped,
		"abandoned": enums.StatusDropped,
	}

	for raw, want := range cases {
		if got := mapLunaBoxStatus(raw); got != want {
			t.Errorf("mapLunaBoxStatus(%q) = %q, want %q", raw, got, want)
		}
	}
}

// LunaBox 的来源名必须走 importer 里唯一的那份映射（source_mapping.go），
// 不能另写一个 switch —— 漏项的后果是静默归成 local 或另一个来源。
func TestLunaBoxSourceUsesSharedMapping(t *testing.T) {
	// 手机版支持的来源全集：备份里出现这些值时必须原样落到对应来源。
	cases := map[string]enums.SourceType{
		"vndb":       enums.VNDB,
		"bangumi":    enums.Bangumi,
		"ymgal":      enums.Ymgal,
		"hikarinagi": enums.Hikarinagi,
		"nextmoe":    enums.NextMoe,
		"steam":      enums.Steam,
		"dlsite":     enums.DLsite,
		"touchgal":   enums.TouchGal,
		"local":      enums.Local,
	}
	for raw, want := range cases {
		if got := mapExternalSourceName(raw); got != want {
			t.Errorf("mapExternalSourceName(%q) = %q, want %q", raw, got, want)
		}
	}

	// bangumi_mirror 必须折叠成 bangumi（与手机版一致：它从不写进缓存来源）
	if got := mapExternalSourceName("bangumi_mirror"); got != enums.Bangumi {
		t.Errorf("bangumi_mirror 应折叠成 bangumi，实际 %q", got)
	}
	if got := mapExternalSourceName("  VNDB  "); got != enums.VNDB {
		t.Errorf("大小写/空白应被忽略，实际 %q", got)
	}
	if got := mapExternalSourceName("some_new_source"); got != enums.Local {
		t.Errorf("未知来源应兜底为 local，实际 %q", got)
	}
}

// 时间戳解析：DuckDB 导出的是 PostgreSQL 风格、带时区偏移、小数位数不定。
func TestParseLunaBoxTime(t *testing.T) {
	cases := map[string]bool{
		"2026-07-16 11:56:43.468941+08": true,
		"2026-07-16 18:39:05.40827+08":  true, // 5 位小数
		"2026-07-16 11:55:03.747711+08": true,
		"2026-07-16 11:55:03+08":        true, // 无小数
		"2026-07-16 11:55:03":           true, // 无时区
		"2026-07-16":                    true,
		"":                              false,
		"null":                          false,
		"看不懂的东西":                        false,
	}
	for raw, wantValid := range cases {
		parsed := parseLunaBoxTime(raw)
		if parsed.IsZero() == wantValid {
			t.Errorf("parseLunaBoxTime(%q) = %v, 期望有效=%v", raw, parsed, wantValid)
		}
	}

	// 带 +08 的时间必须按 +08 解释，而不是被当成 UTC
	parsed := parseLunaBoxTime("2026-07-16 11:56:43.468941+08")
	want := time.Date(2026, 7, 16, 11, 56, 43, 468941000, time.FixedZone("", 8*3600))
	if !parsed.Equal(want) {
		t.Errorf("带时区的时间解析错误: got %v, want %v", parsed, want)
	}
}

// 布尔与数值解析：DuckDB 导出小写 true/false，空值按零值。
func TestParseLunaBoxScalars(t *testing.T) {
	for raw, want := range map[string]bool{
		"true": true, "t": true, "1": true, "yes": true, "TRUE": true,
		"false": false, "f": false, "0": false, "": false, "null": false,
	} {
		if got := parseLunaBoxBool(raw); got != want {
			t.Errorf("parseLunaBoxBool(%q) = %v, want %v", raw, got, want)
		}
	}

	if got := parseLunaBoxFloat("8.32"); got != 8.32 {
		t.Errorf("parseLunaBoxFloat(\"8.32\") = %v", got)
	}
	if got := parseLunaBoxFloat(""); got != 0 {
		t.Errorf("parseLunaBoxFloat(\"\") = %v, want 0", got)
	}
	if got := parseLunaBoxInt("141"); got != 141 {
		t.Errorf("parseLunaBoxInt(\"141\") = %v", got)
	}
	if got := parseLunaBoxInt(""); got != 0 {
		t.Errorf("parseLunaBoxInt(\"\") = %v, want 0", got)
	}
}

// 本地封面引用要能从 /local/covers/<文件名> 还原出 ZIP 内的文件名。
func TestParseLunaBoxLocalCoverFile(t *testing.T) {
	cases := map[string]string{
		"/local/covers/1bcad339-e321-4803-ae3d-06824b79cb50.webp": "1bcad339-e321-4803-ae3d-06824b79cb50.webp",
		"https://t.vndb.org/cv/61/77161.jpg":                      "",
		"":                                                        "",
		"/local/covers/":                                          "",
		"cover.webp":                                              "",
	}
	for raw, want := range cases {
		if got := lunaBoxLocalCoverFile(raw); got != want {
			t.Errorf("lunaBoxLocalCoverFile(%q) = %q, want %q", raw, got, want)
		}
	}
}

// 游玩记录必须换成本机主键，否则会写出一批指向不存在游戏的孤立记录
// （不报错，但游玩时长全丢）。
func TestRekeyLunaBoxSessions(t *testing.T) {
	original := []models.PlaySession{
		{ID: "s1", GameID: "lunabox-uuid", Duration: 141},
		{ID: "s2", GameID: "lunabox-uuid", Duration: 311},
	}

	rekeyed := rekeyLunaBoxSessions(original, "local-uuid")
	if len(rekeyed) != 2 {
		t.Fatalf("记录数 = %d, want 2", len(rekeyed))
	}
	for _, session := range rekeyed {
		if session.GameID != "local-uuid" {
			t.Errorf("game_id = %q, want local-uuid", session.GameID)
		}
	}
	// 不能改动原始切片（同一份存档可能被多次引用）
	if original[0].GameID != "lunabox-uuid" {
		t.Errorf("原始切片被就地修改了: %q", original[0].GameID)
	}
	// 空输入返回 nil，不应产生空切片
	if got := rekeyLunaBoxSessions(nil, "x"); got != nil {
		t.Errorf("空输入应返回 nil，实际 %v", got)
	}
}

// 端到端解析：造一份与 LunaBox 结构一致的 ZIP，验证 CSV（含引号内逗号与换行）、
// 游玩记录、标签与封面都能正确读出。
func TestReadLunaBoxArchiveParsesZipBackup(t *testing.T) {
	const gameID = "1bcad339-e321-4803-ae3d-06824b79cb50"
	const otherID = "eb470d31-d347-43ba-9136-32f295db0c21"

	gamesCSV := "id,name,cover_url,company,summary,rating,release_date,path,status,source_type,source_id,is_nsfw,aliases,created_at,updated_at\r\n" +
		gameID + ",超次元恋人！！,/local/covers/" + gameID + ".webp,CRYSTALiA,\"第一行\r\n第二行,带逗号\",8.32,2026-04-24,D:\\galgame\\a.exe,completed,vndb,v60663,false,\"[\"\"别名A\"\",\"\"别名B\"\"]\",2026-07-16 11:56:43.468941+08,2026-07-16 18:39:05.40827+08\r\n" +
		otherID + ",Terraria,,,,\"0.0\",,,playing,steam,1281930,true,[],2026-07-16 11:55:33.749651+08,2026-07-16 11:55:33.749651+08\r\n"

	sessionsCSV := "id,game_id,start_time,end_time,duration,updated_at\r\n" +
		"s1," + gameID + ",2026-07-16 17:34:23.673844+08,2026-07-16 17:36:44.673844+08,141,2026-07-16 17:36:45.161866+08\r\n" +
		"s2," + otherID + ",2026-07-16 17:38:35.503465+08,2026-07-16 17:43:46.503465+08,311,2026-07-16 17:43:47.210083+08\r\n"

	tagsCSV := "id,game_id,name,source,weight,is_spoiler,created_at,updated_at\r\n" +
		"t1," + gameID + ",科幻,user,1.0,false,2026-07-16 11:56:43+08,2026-07-16 11:56:43+08\r\n" +
		"t2," + gameID + ",恋愛,user,0.5,true,2026-07-16 11:56:43+08,2026-07-16 11:56:43+08\r\n"

	zipPath := writeTestLunaBoxZip(t, map[string]string{
		"database/games.csv":         gamesCSV,
		"database/play_sessions.csv": sessionsCSV,
		"database/game_tags.csv":     tagsCSV,
	}, map[string][]byte{
		"covers/" + gameID + ".webp": []byte("fake-webp-bytes"),
	})

	archive, err := readLunaBoxArchive(zipPath)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(archive.entries) != 2 {
		t.Fatalf("游戏数 = %d, want 2", len(archive.entries))
	}

	first := archive.entries[0].game
	if first.Name != "超次元恋人！！" {
		t.Errorf("name = %q", first.Name)
	}
	// 简介里有换行与逗号，属于最容易解析错的一列。
	// 注意 encoding/csv 会把引号内的 \r\n 统一成 \n，这是标准库的行为。
	if first.Summary != "第一行\n第二行,带逗号" {
		t.Errorf("summary 解析错误: %q", first.Summary)
	}
	if first.Rating != 8.32 {
		t.Errorf("rating = %v, want 8.32", first.Rating)
	}
	if first.Company != "CRYSTALiA" {
		t.Errorf("company = %q", first.Company)
	}
	if first.Status != enums.StatusCompleted {
		t.Errorf("status = %q, want completed", first.Status)
	}
	if first.SourceType != enums.VNDB {
		t.Errorf("source_type = %q, want vndb", first.SourceType)
	}
	if first.SourceID != "v60663" {
		t.Errorf("source_id = %q", first.SourceID)
	}
	// 桌面端的 path 是有效的 Windows 路径，必须保留（会被规范化成小写盘符）
	if first.Path != `d:\galgame\a.exe` {
		t.Errorf("path = %q（桌面端应保留启动路径）", first.Path)
	}
	if first.ID == "" || first.ID == gameID {
		t.Errorf("主键应由本机重新生成，实际 %q", first.ID)
	}
	if len(first.Aliases) != 2 || first.Aliases[0] != "别名A" {
		t.Errorf("aliases 解析错误: %v", first.Aliases)
	}
	if archive.entries[0].coverFile != gameID+".webp" {
		t.Errorf("本地封面文件名 = %q", archive.entries[0].coverFile)
	}

	// 第二条：空字段不能变成脏值，布尔 true 要读对
	second := archive.entries[1].game
	if second.Company != "" || second.Summary != "" {
		t.Errorf("空列应保持为空: company=%q summary=%q", second.Company, second.Summary)
	}
	if !second.IsNSFW {
		t.Errorf("is_nsfw = true 应被读出")
	}
	if second.Status != enums.StatusPlaying {
		t.Errorf("status = %q, want playing", second.Status)
	}
	if archive.entries[1].coverFile != "" {
		t.Errorf("无本地封面时 coverFile 应为空，实际 %q", archive.entries[1].coverFile)
	}

	// 游玩记录：duration 是秒，直接透传（不做毫秒换算）
	sessions := archive.sessions[gameID]
	if len(sessions) != 1 {
		t.Fatalf("游玩记录数 = %d, want 1", len(sessions))
	}
	if sessions[0].Duration != 141 {
		t.Errorf("duration = %d, want 141（秒）", sessions[0].Duration)
	}
	if sessions[0].GameID != gameID {
		t.Errorf("game_id = %q", sessions[0].GameID)
	}
	if sessions[0].StartTime.IsZero() {
		t.Errorf("start_time 未解析")
	}

	// 标签：按 game_id 归组
	tags := archive.entries[0].tags
	if len(tags) != 2 {
		t.Fatalf("标签数 = %d, want 2", len(tags))
	}
	if tags[0].Name != "科幻" || tags[1].IsSpoiler != true {
		t.Errorf("标签解析错误: %+v", tags)
	}

	// 封面字节应被解出来
	if len(archive.covers[gameID+".webp"]) == 0 {
		t.Errorf("covers 映射里没有该封面")
	}

	// 游玩记录靠 LunaBox 的 id 关联（本机主键是新生成的 uuid）
	if archive.entries[0].lunaBoxID != gameID {
		t.Errorf("lunaBoxID = %q, want %q", archive.entries[0].lunaBoxID, gameID)
	}
	rekeyed := rekeyLunaBoxSessions(archive.sessions[archive.entries[0].lunaBoxID], first.ID)
	if len(rekeyed) != 1 || rekeyed[0].GameID != first.ID {
		t.Errorf("rekey 后无法关联到本机主键: %+v", rekeyed)
	}
}

// 缺列时不该 panic 也不该产生脏值：LunaBox 各版本列数不同（v1.12.1 才有
// is_nsfw / aliases / game_directory 等列）。
func TestReadLunaBoxArchiveToleratesMissingColumns(t *testing.T) {
	// 只有最古老的几列
	oldGamesCSV := "id,name,cover_url,status,source_type\r\n" +
		"g1,老备份游戏,,on_hold,bangumi\r\n"

	zipPath := writeTestLunaBoxZip(t, map[string]string{
		"database/games.csv": oldGamesCSV,
	}, nil)

	archive, err := readLunaBoxArchive(zipPath)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(archive.entries) != 1 {
		t.Fatalf("游戏数 = %d, want 1", len(archive.entries))
	}
	game := archive.entries[0].game
	if game.Name != "老备份游戏" {
		t.Errorf("name = %q", game.Name)
	}
	if game.Status != enums.StatusOnHold {
		t.Errorf("on_hold 应映射为 onhold，实际 %q", game.Status)
	}
	if game.SourceType != enums.Bangumi {
		t.Errorf("source_type = %q", game.SourceType)
	}
	if game.IsNSFW || game.Rating != 0 || game.Company != "" {
		t.Errorf("缺失列应保持零值: %+v", game)
	}
	if game.CreatedAt.IsZero() {
		t.Errorf("created_at 缺失时应兜底为当前时间，不能是零值")
	}
}

// 没有 games.csv 时要给出可读的错误，而不是静默当成空备份。
func TestReadLunaBoxArchiveRequiresGamesCSV(t *testing.T) {
	zipPath := writeTestLunaBoxZip(t, map[string]string{
		"database/play_sessions.csv": "id,game_id\r\n",
	}, nil)

	if _, err := readLunaBoxArchive(zipPath); err == nil {
		t.Fatalf("缺少 games.csv 时应报错")
	}
}

// 非 ZIP 文件要给出「需要 .zip 备份文件」的提示。
func TestReadLunaBoxArchiveRejectsNonZip(t *testing.T) {
	dir := t.TempDir()
	notZip := filepath.Join(dir, "lunabox.db")
	if err := os.WriteFile(notZip, []byte("not a zip at all"), 0o644); err != nil {
		t.Fatalf("准备测试文件失败: %v", err)
	}
	if _, err := readLunaBoxArchive(notZip); err == nil {
		t.Fatalf("非 ZIP 文件应报错")
	}
}

// writeTestLunaBoxZip 在临时目录造一份 LunaBox 结构的 ZIP。
func writeTestLunaBoxZip(t *testing.T, textFiles map[string]string, binaryFiles map[string][]byte) string {
	t.Helper()

	zipPath := filepath.Join(t.TempDir(), "lunabox_test.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("创建测试 ZIP 失败: %v", err)
	}
	defer func() { _ = file.Close() }()

	writer := zip.NewWriter(file)
	for name, content := range textFiles {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("写入 %s 失败: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("写入 %s 失败: %v", name, err)
		}
	}
	for name, content := range binaryFiles {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("写入 %s 失败: %v", name, err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("写入 %s 失败: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("关闭测试 ZIP 失败: %v", err)
	}
	return zipPath
}
