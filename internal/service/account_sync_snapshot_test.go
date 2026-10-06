package service

import "testing"

// 云同步「本地库是否为空」的判定，对齐手机版 SyncManager.isSnapshotEmpty：
// 只看 games 数组为不为空，不看游玩记录 / 元数据缓存。
//
// 这条判定决定「清空游戏库后点同步」是下载还是上传 —— 判错就会用空库覆盖云端。
func TestSnapshotIsEmpty(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		payload string
		want    bool
	}{
		"没有 games 键":   {`{"app":"YukiHub","schema":5}`, true},
		"games 为空数组":   {`{"games":[]}`, true},
		"games 为 null": {`{"games":null}`, true},
		"games 有内容":    {`{"games":[{"title":"游戏 A"}]}`, false},
		"只有游玩记录不算空":    {`{"games":[],"play_sessions":[{"session_uuid":"u1"}]}`, true},
		"只有元数据缓存不算空":   {`{"games":[],"metadata_cache":[{"game_local_id":1}]}`, true},
		"非法 JSON 视为空":  {`{`, true},
		"空字节":          {``, true},
	}

	for name, testCase := range cases {
		if got := snapshotIsEmpty([]byte(testCase.payload)); got != testCase.want {
			t.Errorf("%s: snapshotIsEmpty(%s) = %v, want %v", name, testCase.payload, got, testCase.want)
		}
	}
}

// countSnapshotEntries 用于界面提示的条目计数，必须能数出 games 与 play_sessions。
func TestCountSnapshotEntries(t *testing.T) {
	t.Parallel()

	games, sessions := countSnapshotEntries([]byte(
		`{"games":[{"title":"A"},{"title":"B"}],"play_sessions":[{"session_uuid":"u1"}]}`,
	))
	if games != 2 || sessions != 1 {
		t.Fatalf("countSnapshotEntries = (%d, %d), want (2, 1)", games, sessions)
	}

	if games, sessions := countSnapshotEntries([]byte(`{`)); games != 0 || sessions != 0 {
		t.Fatalf("非法 JSON 应返回 (0, 0)，得到 (%d, %d)", games, sessions)
	}
}
