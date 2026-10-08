package service

import (
	"testing"

	"yukihub/internal/appconf"
	"yukihub/internal/common/enums"
)

// 「当前资料源」白名单必须与手机版 importSnapshot 接受的取值完全一致。
//
// 桌面端自己的 IsSupportedMetadataSource 更宽（含 steam / dlsite / touchgal /
// erogamescape），那些值手机端不认识；当成跨端全局偏好落库只会白写一次。
func TestIsSelectableMetadataSourceMatchesMobileWhitelist(t *testing.T) {
	t.Parallel()

	for _, accepted := range []string{
		"vndb", "bangumi", "bangumi_mirror", "ymgal", "hikarinagi", "nextmoe",
		"VNDB", "  NextMoe  ",
	} {
		if !appconf.IsSelectableMetadataSource(accepted) {
			t.Errorf("IsSelectableMetadataSource(%q) = false, want true", accepted)
		}
	}

	for _, rejected := range []string{
		"", "   ", "steam", "dlsite", "touchgal", "erogamescape", "local", "bogus",
	} {
		if appconf.IsSelectableMetadataSource(rejected) {
			t.Errorf("IsSelectableMetadataSource(%q) = true, want false", rejected)
		}
	}
}

// 导入快照时的资料源采纳决策。白名单外 / 缺失 / 与当前一致都不许动配置。
func TestResolveImportedMetadataSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		current    enums.SourceType
		incoming   string
		wantValue  enums.SourceType
		wantChange bool
	}{
		{"采纳对端资料源", enums.VNDB, "nextmoe", enums.NextMoe, true},
		{"大小写与空白归一", enums.VNDB, "  NextMoe  ", enums.NextMoe, true},
		{"改回 vndb", enums.NextMoe, "vndb", enums.VNDB, true},
		{"与当前一致不动", enums.NextMoe, "nextmoe", enums.NextMoe, false},
		{"空串不动", enums.VNDB, "", enums.VNDB, false},
		{"纯空白不动", enums.VNDB, "   ", enums.VNDB, false},
		{"桌面端独有来源不采纳", enums.VNDB, "steam", enums.VNDB, false},
		{"未知值不采纳", enums.VNDB, "not-a-source", enums.VNDB, false},
	}
	for _, tc := range cases {
		gotValue, gotChange := resolveImportedMetadataSource(tc.current, tc.incoming)
		if gotValue != tc.wantValue || gotChange != tc.wantChange {
			t.Errorf("%s: resolveImportedMetadataSource(%q, %q) = (%q, %v), want (%q, %v)",
				tc.name, tc.current, tc.incoming, gotValue, gotChange, tc.wantValue, tc.wantChange)
		}
	}
}
