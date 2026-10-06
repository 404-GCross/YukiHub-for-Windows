package service

import (
	"testing"
	"time"

	"yukihub/internal/appconf"
)

// 进程退出路径在退出瞬间记下 exitTimeNanos；结算必须用它当 endTime，而不是
// 继任者检测 / 音频恢复完成后的「现在」—— 否则那几秒宽限会被算进游玩时长。
// 真实场景：AGES 引擎在菜单里停留任意久后自重启，兜底监控的进程退出 →
// 继任者检测宽限最多 6 秒，时长不能跟着膨胀。
func TestFinalizeUsesRecordedProcessExitTime(t *testing.T) {
	startService, db := setupManualPlaySessionTest(t, &appconf.AppConfig{})

	started, err := startService.StartManualPlaySession(manualSessionTestGameID)
	if err != nil {
		t.Fatalf("StartManualPlaySession: %v", err)
	}
	if !started {
		t.Fatal("首次开始计时应当返回 started=true")
	}

	session := startService.getActiveSession(manualSessionTestGameID)
	// 会话开始于 2 小时前，进程在 90 分钟前退出 → 真实时长 = 30 分钟 = 1800 秒。
	// 若 endTime 错取「现在」，落库时长会变成 ~7200 秒。
	session.startTime = time.Now().Add(-2 * time.Hour)
	session.exitTimeNanos.Store(time.Now().Add(-90 * time.Minute).UnixNano())

	if err := startService.EndCurrentPlaySession(manualSessionTestGameID); err != nil {
		t.Fatalf("EndCurrentPlaySession: %v", err)
	}

	recorded := totalRecordedSeconds(t, db)
	if recorded < 1700 || recorded > 1900 {
		t.Fatalf("落库时长 = %d 秒，期望约 1800 秒（进程真实退出时刻），endTime 取错了", recorded)
	}
}
