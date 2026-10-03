package nativenotify

import "testing"

// 手工验证用：跑起来会在桌面右下角真的弹一条通知。
//
// 之所以值得留一个会弹窗的测试：Shell_NotifyIcon 的结构体布局 / cbSize 一旦
// 写错，返回值只告诉你「失败」，不会有任何提示。这里把「创建 + 发送」整条
// 通路跑通才算数。
//
// 没有桌面会话的环境（CI）会拿不到窗口，这时跳过而不是失败。
func TestNotifierSendsNotification(t *testing.T) {
	notifier, err := New()
	if err != nil {
		t.Skipf("当前环境没有桌面会话，跳过：%v", err)
	}
	defer notifier.Close()

	if err := notifier.Notify("YukiHub", "系统通知自检：如果你看到这条气泡，说明通路是好的"); err != nil {
		t.Fatalf("发送系统通知失败：%v", err)
	}

	// 连发两条，验证复用同一个托盘图标不会互相干扰
	if err := notifier.Notify("YukiHub", "第二条：确认可以连续发"); err != nil {
		t.Fatalf("连续发送失败：%v", err)
	}
}

func TestNotifierCloseIsIdempotent(t *testing.T) {
	notifier, err := New()
	if err != nil {
		t.Skipf("当前环境没有桌面会话，跳过：%v", err)
	}

	notifier.Close()
	// 关闭后再发应当报错而不是 panic
	if err := notifier.Notify("YukiHub", "关闭后不该成功"); err == nil {
		t.Error("关闭后发送应当报错")
	}
	// 重复关闭不该 panic（应用退出路径可能被调用多次）
	notifier.Close()
}
