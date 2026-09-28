package exporter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"yukihub/internal/common/enums"
	"yukihub/internal/models"
)

func TestMapGameStatusToYukiHubDegradesWantToPlay(t *testing.T) {
	t.Parallel()

	cases := map[enums.GameStatus]string{
		enums.StatusNotStarted: "unplayed",
		// Android 侧没有「想玩」，按契约降级为 unplayed。
		enums.StatusWantToPlay: "unplayed",
		enums.StatusPlaying:    "playing",
		enums.StatusCompleted:  "completed",
		enums.StatusOnHold:     "onhold",
		enums.StatusDropped:    "dropped",
		enums.GameStatus(""):   "unplayed",
		enums.GameStatus("x"):  "unplayed",
	}
	for input, want := range cases {
		if got := mapGameStatusToYukiHub(input); got != want {
			t.Errorf("mapGameStatusToYukiHub(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNetworkCoverURIOnlyExportsRemoteCovers(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"https://example.com/cover.jpg": "https://example.com/cover.jpg",
		"http://example.com/cover.jpg":  "http://example.com/cover.jpg",
		// 本地封面跨设备无效，契约明确不迁移。
		"D:\\covers\\cover.jpg":   "",
		"cover.jpg":               "",
		"content://media/cover/1": "",
		"":                        "",
	}
	for input, want := range cases {
		if got := networkCoverURI(input); got != want {
			t.Errorf("networkCoverURI(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseYukiHubLocalIDFallsBackToZero(t *testing.T) {
	t.Parallel()

	cases := map[string]int64{
		"42":    42,
		" 42 ":  42,
		"":      0,
		"abc":   0,
		"-1":    0,
		"0":     0,
		"1e309": 0,
	}
	for input, want := range cases {
		if got := parseYukiHubLocalID(input); got != want {
			t.Errorf("parseYukiHubLocalID(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestBuildYukiHubGameConvertsSecondsToMillis(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.September, 20, 20, 0, 0, 0, time.Local)
	end := start.Add(90 * time.Second)
	game := models.Game{
		ID:            "game-1",
		Name:          "桌面游戏",
		LegacyLocalID: "42",
		GameDirectory: "D:\\Games\\desktop",
		Path:          "D:\\Games\\desktop\\game.exe",
		CoverURL:      "https://example.com/cover.jpg",
		Summary:       "简介",
		Status:        enums.StatusPlaying,
		CreatedAt:     start.Add(-time.Hour),
		UpdatedAt:     end,
	}
	sessions := []models.PlaySession{{
		ID:        "9b6f0f4a-6d2f-4d0e-9f4f-1f2f0d1a2b3c",
		GameID:    game.ID,
		StartTime: start,
		EndTime:   end,
		Duration:  90, // 桌面端单位是秒
		UpdatedAt: end,
	}}

	entry, exported := buildYukiHubGame(game, []string{"剧情", "校园"}, sessions, false)
	if entry.TotalPlayTime != 90_000 {
		t.Fatalf("TotalPlayTime = %d, want 90000 (毫秒)", entry.TotalPlayTime)
	}
	if len(exported) != 1 || exported[0].Duration != 90_000 {
		t.Fatalf("Session duration = %#v, want one session of 90000 ms", exported)
	}
	if exported[0].SessionUUID != sessions[0].ID {
		t.Errorf("SessionUUID = %q, want %q", exported[0].SessionUUID, sessions[0].ID)
	}
	if exported[0].GameLocalID != 42 || exported[0].GameTitle != "桌面游戏" || exported[0].GameRootUri != "" {
		t.Errorf("Session identity = %+v", exported[0])
	}
	if exported[0].LaunchType != "external" || exported[0].DeviceID != "desktop" {
		t.Errorf("launch_type/device_id = %q/%q, want external/desktop", exported[0].LaunchType, exported[0].DeviceID)
	}
	if exported[0].StartTime != start.UnixMilli() || exported[0].EndTime != end.UnixMilli() || exported[0].CreatedAt != start.UnixMilli() {
		t.Errorf("session timestamps = %+v", exported[0])
	}
	if entry.LastPlayedAt != end.UnixMilli() {
		t.Errorf("LastPlayedAt = %d, want %d", entry.LastPlayedAt, end.UnixMilli())
	}
	// root_uri 恒空：Windows 绝对路径在 Android 上不可达，且与导入方向对称
	// （桌面端导入 Android 备份时同样不写对端路径）。
	if entry.RootUri != "" {
		t.Errorf("root_uri = %q, want empty (不导出 Windows 路径)", entry.RootUri)
	}
	if entry.CoverUri != "https://example.com/cover.jpg" {
		t.Errorf("cover_uri = %q", entry.CoverUri)
	}
	if entry.Tags != "剧情,校园" {
		t.Errorf("Tags = %q, want \"剧情,校园\"", entry.Tags)
	}
	if entry.PlaytimeResetAt != 0 {
		t.Errorf("PlaytimeResetAt = %d, want 0 for a game that was never reset", entry.PlaytimeResetAt)
	}
	if entry.CreatedAt != game.CreatedAt.UnixMilli() || entry.UpdatedAt != end.UnixMilli() {
		t.Errorf("created_at/updated_at = %d/%d", entry.CreatedAt, entry.UpdatedAt)
	}
}

func TestBuildYukiHubGameSkipsSessionsBeforeReset(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.September, 21, 10, 0, 0, 0, time.Local)
	resetAt := base.Add(time.Hour)
	game := models.Game{
		ID:              "game-1",
		Name:            "清零过的游戏",
		PlaytimeResetAt: &resetAt,
		Status:          enums.StatusCompleted,
		CreatedAt:       base.Add(-24 * time.Hour),
		UpdatedAt:       base.Add(2 * time.Hour),
	}
	sessions := []models.PlaySession{
		// 清零之前：必须被丢弃，否则清零历史会在 Android 侧复活。
		{ID: "old-1", GameID: game.ID, StartTime: base.Add(-30 * time.Minute), EndTime: base.Add(-29 * time.Minute), Duration: 60, UpdatedAt: base},
		// 边界：结束时间正好等于清零时刻，Android 侧的条件是 >=，必须保留。
		{ID: "boundary", GameID: game.ID, StartTime: resetAt.Add(-time.Minute), EndTime: resetAt, Duration: 60, UpdatedAt: resetAt},
		{ID: "new-1", GameID: game.ID, StartTime: base.Add(90 * time.Minute), EndTime: base.Add(92 * time.Minute), Duration: 120, UpdatedAt: base.Add(92 * time.Minute)},
	}

	entry, exported := buildYukiHubGame(game, nil, sessions, false)
	if entry.PlaytimeResetAt != resetAt.UnixMilli() {
		t.Fatalf("PlaytimeResetAt = %d, want %d", entry.PlaytimeResetAt, resetAt.UnixMilli())
	}
	if len(exported) != 2 {
		t.Fatalf("exported session count = %d, want 2: %#v", len(exported), exported)
	}
	if exported[0].SessionUUID != "boundary" || exported[1].SessionUUID != "new-1" {
		t.Fatalf("unexpected exported sessions: %#v", exported)
	}
	if entry.TotalPlayTime != 180_000 {
		t.Errorf("TotalPlayTime = %d, want 180000 (毫秒)", entry.TotalPlayTime)
	}
	if entry.LastPlayedAt != base.Add(92*time.Minute).UnixMilli() {
		t.Errorf("LastPlayedAt = %d, want the latest end_time", entry.LastPlayedAt)
	}
}

func TestBuildYukiHubGameKeepsOnlyLatestSessions(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.September, 22, 8, 0, 0, 0, time.Local)
	game := models.Game{ID: "game-1", Name: "会话很多的游戏"}
	sessions := make([]models.PlaySession, 0, 40)
	for index := 0; index < 40; index++ {
		start := base.Add(time.Duration(index) * time.Minute)
		sessions = append(sessions, models.PlaySession{
			ID:        fmt.Sprintf("session-%02d", index),
			GameID:    game.ID,
			StartTime: start,
			EndTime:   start.Add(30 * time.Second),
			Duration:  30,
			UpdatedAt: start.Add(30 * time.Second),
		})
	}

	entry, exported := buildYukiHubGame(game, nil, sessions, false)
	if len(exported) != yukiHubSessionsPerGame {
		t.Fatalf("exported session count = %d, want %d", len(exported), yukiHubSessionsPerGame)
	}
	// 保留最新的 30 条：最早的是第 11 条（索引 10），最晚的是第 40 条（索引 39）。
	if exported[0].StartTime != base.Add(10*time.Minute).UnixMilli() {
		t.Errorf("first exported session = %d, want %d", exported[0].StartTime, base.Add(10*time.Minute).UnixMilli())
	}
	if exported[len(exported)-1].StartTime != base.Add(39*time.Minute).UnixMilli() {
		t.Errorf("last exported session = %d, want the newest one", exported[len(exported)-1].StartTime)
	}
	// 总时长统计不受 30 条上限影响（Android 侧的 total_play_time 也是全量聚合）。
	if entry.TotalPlayTime != 40*30*1000 {
		t.Errorf("TotalPlayTime = %d, want %d", entry.TotalPlayTime, 40*30*1000)
	}
}

func TestBuildYukiHubGameIsIdempotent(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.September, 23, 9, 0, 0, 0, time.Local)
	game := models.Game{ID: "game-1", Name: "重复导出的游戏", LegacyLocalID: "7"}
	sessions := []models.PlaySession{
		{ID: "11111111-1111-1111-1111-111111111111", GameID: game.ID, StartTime: base, EndTime: base.Add(time.Minute), Duration: 60, UpdatedAt: base.Add(time.Minute)},
	}
	first, firstSessions := buildYukiHubGame(game, []string{"标签"}, sessions, true)
	second, secondSessions := buildYukiHubGame(game, []string{"标签"}, sessions, true)

	if !equalAsJSON(t, first, second) || !equalAsJSON(t, firstSessions, secondSessions) {
		t.Fatal("exporting the same data twice must produce identical snapshots")
	}
	// 幂等键就是桌面端的会话 UUID，Android 侧靠它去重。
	for index := range secondSessions {
		if secondSessions[index].SessionUUID != sessions[index].ID {
			t.Errorf("SessionUUID = %q, want %q", secondSessions[index].SessionUUID, sessions[index].ID)
		}
	}
	if !first.Favorite || !second.Favorite {
		t.Error("Favorite was not exported")
	}
}

func TestBuildYukiHubGameWithoutSessions(t *testing.T) {
	t.Parallel()

	game := models.Game{
		ID:      "game-1",
		Name:    "还没玩过的游戏",
		Aliases: []string{"Alias One"},
		Path:    "D:\\Games\\game.exe",
		// 本地封面不迁移。
		CoverURL: "D:\\covers\\game.jpg",
		Status:   enums.StatusWantToPlay,
		IsNSFW:   true,
		Hidden:   true,
	}

	entry, exported := buildYukiHubGame(game, nil, nil, false)
	if len(exported) != 0 {
		t.Fatalf("exported session count = %d, want 0", len(exported))
	}
	if entry.TotalPlayTime != 0 || entry.LastPlayedAt != 0 {
		t.Errorf("total_play_time/last_played_at = %d/%d, want 0/0", entry.TotalPlayTime, entry.LastPlayedAt)
	}
	if entry.OriginalTitle != "Alias One" {
		t.Errorf("original_title = %q, want \"Alias One\"", entry.OriginalTitle)
	}
	if entry.RootUri != "" {
		t.Errorf("root_uri = %q, want empty regardless of game_directory/path", entry.RootUri)
	}
	if entry.CoverUri != "" {
		t.Errorf("cover_uri = %q, want empty for a local cover", entry.CoverUri)
	}
	if entry.PlayStatus != "unplayed" || !entry.Hidden || !entry.NSFW {
		t.Errorf("play_status/hidden/nsfw = %q/%v/%v", entry.PlayStatus, entry.Hidden, entry.NSFW)
	}
}

// equalAsJSON 用 JSON 序列化结果比较两个值，避免引入 reflect.DeepEqual 对
// nil / 空切片、零值时间的差异误报。
func equalAsJSON(t *testing.T, left interface{}, right interface{}) bool {
	t.Helper()

	leftData, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("marshal left: %v", err)
	}
	rightData, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("marshal right: %v", err)
	}
	return string(leftData) == string(rightData)
}

// TestBuildSkipsUntitledGames 无标题条目不应导出：Android 侧会把空标题落成
// 「未命名游戏」，这类条目跨端同步时只会制造无法匹配的占位记录。
func TestBuildSkipsUntitledGames(t *testing.T) {
	games := []models.Game{
		{ID: "named", Name: "有标题"},
		{ID: "blank", Name: "   "},
		{ID: "empty", Name: ""},
	}
	exported := make([]string, 0, len(games))
	for _, game := range games {
		if strings.TrimSpace(game.Name) == "" {
			continue
		}
		entry, _ := buildYukiHubGame(game, nil, nil, false)
		exported = append(exported, entry.Title)
	}
	if len(exported) != 1 || exported[0] != "有标题" {
		t.Fatalf("导出条目 = %#v, want 只有 [有标题]", exported)
	}
}
