//go:build windows

// Package winwindow 放少量「Wails 没提供、但窗口行为需要」的 Win32 小工具。
package winwindow

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
)

// gwlExStyle 是扩展样式在窗口结构里的偏移（GWL_EXSTYLE = -20）。
// 用变量而不是常量：GetWindowLongPtr 要的是 LONG_PTR，负值常量转 uintptr
// 会在编译期报溢出，必须先落到一个带符号的变量上（转换时按补码展开）。
var gwlExStyle int32 = -20

const (
	// WS_EX_NOACTIVATE：窗口被显示/被点击都不会变成前台窗口。
	//
	// 对通知浮层是硬需求 —— Wails 的 show() 走 SW_SHOW，而 SW_SHOW 会激活窗口。
	// 抢焦点等于把游戏踢出前台，很多游戏会因此自动暂停甚至最小化，
	// 一个「通知」把正在玩的东西打断就本末倒置了。
	wsExNoActivate = 0x08000000
)

// MakeNonActivating 让窗口显示与点击都不抢焦点（加 WS_EX_NOACTIVATE）。
//
// 只对通知浮层这类「只读」窗口合适：窗口不会获得键盘焦点，所以放里面等按键
// 的界面（比如好友栏按 Esc 收起）不能用它。
func MakeNonActivating(handle unsafe.Pointer) error {
	hwnd := uintptr(handle)
	if hwnd == 0 {
		return fmt.Errorf("窗口句柄为空")
	}

	style, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlExStyle))
	if style&wsExNoActivate != 0 {
		return nil
	}

	// SetWindowLongPtr 的返回值是旧样式，无法据此判断成败（旧值可能就是 0），
	// 所以改完再读一次确认。
	_, _, _ = procSetWindowLongPtrW.Call(
		hwnd, uintptr(gwlExStyle), style|wsExNoActivate,
	)
	updated, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlExStyle))
	if updated&wsExNoActivate == 0 {
		return fmt.Errorf("设置 WS_EX_NOACTIVATE 失败")
	}
	return nil
}
