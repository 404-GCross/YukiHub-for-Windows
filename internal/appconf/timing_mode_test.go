package appconf

import "testing"

// 老配置没有 play_timing_mode 字段（反序列化后为空串），归一化后必须是
// 默认的进程监测 —— 升级用户行为不能变。
func TestNormalizePlayTimingModeDefaults(t *testing.T) {
	config := defaultAppConfig()
	config.PlayTimingMode = ""
	if got := NormalizePlayTimingMode(config.PlayTimingMode); got != DefaultPlayTimingMode {
		t.Fatalf("empty mode normalized to %q, want %q", got, DefaultPlayTimingMode)
	}

	config.PlayTimingMode = "manual"
	if got := NormalizePlayTimingMode(config.PlayTimingMode); got != PlayTimingModeManual {
		t.Fatalf("manual normalized to %q, want %q", got, PlayTimingModeManual)
	}

	config.PlayTimingMode = "nonsense"
	if got := NormalizePlayTimingMode(config.PlayTimingMode); got != PlayTimingModeProcess {
		t.Fatalf("bogus normalized to %q, want %q", got, PlayTimingModeProcess)
	}
}
