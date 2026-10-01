package importer

import (
	"testing"

	"yukihub/internal/common/enums"
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

// LunaBox 的来源取值是本仓库的子集，直接透传；未知值兜底为 local。
func TestLunaBoxSourceTypePassesThroughKnownSources(t *testing.T) {
	known := []string{
		"local", "bangumi", "vndb", "ymgal", "steam",
		"dlsite", "touchgal", "hikarinagi", "erogamescape",
		// 本仓库多出来的两个：读本仓库自己的库文件时会遇到
		"bangumi_mirror", "nextmoe",
	}
	for _, raw := range known {
		if got, want := lunaBoxSourceType(raw), enums.SourceType(raw); got != want {
			t.Errorf("lunaBoxSourceType(%q) = %q, want %q", raw, got, want)
		}
	}

	// 大小写与空白不应影响结果
	if got := lunaBoxSourceType("  VNDB  "); got != enums.VNDB {
		t.Errorf("lunaBoxSourceType(\"  VNDB  \") = %q, want vndb", got)
	}

	for _, raw := range []string{"", "unknown", "some_new_source"} {
		if got := lunaBoxSourceType(raw); got != enums.Local {
			t.Errorf("lunaBoxSourceType(%q) = %q, want local", raw, got)
		}
	}
}

// 按列名取值时不该因为缺列而 panic 或产生脏值。
func TestLunaBoxRowValueHelpersTolerateMissingColumns(t *testing.T) {
	empty := map[string]any{}

	if got := lunaBoxString(empty, "name"); got != "" {
		t.Errorf("lunaBoxString(missing) = %q, want \"\"", got)
	}
	if got := lunaBoxFloat(empty, "rating"); got != 0 {
		t.Errorf("lunaBoxFloat(missing) = %v, want 0", got)
	}
	if got := lunaBoxBool(empty, "is_nsfw"); got {
		t.Errorf("lunaBoxBool(missing) = true, want false")
	}
	if got := lunaBoxTime(empty, "created_at"); !got.IsZero() {
		t.Errorf("lunaBoxTime(missing) = %v, want zero", got)
	}
	if got := lunaBoxTimePtr(empty, "last_played_at"); got != nil {
		t.Errorf("lunaBoxTimePtr(missing) = %v, want nil", got)
	}

	// nil 与常见类型都应能被正确读出
	row := map[string]any{
		"name":     []byte("  测试游戏  "),
		"rating":   int64(9),
		"is_nsfw":  int64(1),
		"hidden":   nil,
		"duration": "3600",
	}
	if got := lunaBoxString(row, "name"); got != "测试游戏" {
		t.Errorf("lunaBoxString([]byte) = %q, want 测试游戏", got)
	}
	if got := lunaBoxFloat(row, "rating"); got != 9 {
		t.Errorf("lunaBoxFloat(int64) = %v, want 9", got)
	}
	if got := lunaBoxFloat(row, "duration"); got != 3600 {
		t.Errorf("lunaBoxFloat(string) = %v, want 3600", got)
	}
	if !lunaBoxBool(row, "is_nsfw") {
		t.Errorf("lunaBoxBool(int64 1) = false, want true")
	}
	if lunaBoxBool(row, "hidden") {
		t.Errorf("lunaBoxBool(nil) = true, want false")
	}
}
