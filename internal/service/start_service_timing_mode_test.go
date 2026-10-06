package service

import (
	"testing"

	"yukihub/internal/appconf"
)

// 计时模式归一化：空串（老配置没有这个字段）与未知值一律回退「进程监测」，
// 保证升级用户的行为完全不变。
func TestNormalizePlayTimingMode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", appconf.PlayTimingModeProcess},
		{"process", appconf.PlayTimingModeProcess},
		{"PROCESS", appconf.PlayTimingModeProcess},
		{"  process  ", appconf.PlayTimingModeProcess},
		{"manual", appconf.PlayTimingModeManual},
		{"Manual", appconf.PlayTimingModeManual},
		{"something-else", appconf.PlayTimingModeProcess},
	}
	for _, c := range cases {
		if got := appconf.NormalizePlayTimingMode(c.in); got != c.want {
			t.Errorf("NormalizePlayTimingMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// usesManualTimingMode 只认归一化后的 manual；nil 配置（部分测试环境）按默认
// 进程监测走。
func TestUsesManualTimingMode(t *testing.T) {
	s := NewStartService()
	if s.usesManualTimingMode() {
		t.Fatal("nil config should default to process timing mode")
	}

	s.config = &appconf.AppConfig{PlayTimingMode: appconf.PlayTimingModeProcess}
	if s.usesManualTimingMode() {
		t.Fatal("process mode should not be manual")
	}

	s.config = &appconf.AppConfig{PlayTimingMode: appconf.PlayTimingModeManual}
	if !s.usesManualTimingMode() {
		t.Fatal("manual mode should be detected")
	}

	// 脏值：归一化后不是 manual
	s.config = &appconf.AppConfig{PlayTimingMode: "bogus"}
	if s.usesManualTimingMode() {
		t.Fatal("bogus value should normalize to process mode")
	}
}

// 手动计时模式下，结算必须走墙钟（activeTrackStarted 恒 false），绝不能落进
// `<60 秒删除` 的坑 —— 那会把手动的整场游玩记录静默删掉。
func TestManualTimingModeSessionUsesWallClock(t *testing.T) {
	s := NewStartService()
	s.config = &appconf.AppConfig{PlayTimingMode: appconf.PlayTimingModeManual}

	session := &activePlaySession{}
	// 手动模式启动的会话不会启动活跃追踪器
	if s.usesActiveTimeTracking(session) {
		t.Fatal("manual timing session must not use active time tracking")
	}
	if mode := s.runtimeTimingMode(session); mode != GameRuntimeTimingModeWallClock {
		t.Fatalf("manual timing session timing mode = %q, want wall-clock", mode)
	}
	if s.runtimeActiveSeconds(session, 42) != nil {
		t.Fatal("manual timing session must not report active seconds")
	}
}
