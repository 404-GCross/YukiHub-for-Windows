//go:build windows

// Package winwindow 放少量「Wails 没提供、但窗口行为需要」的 Win32 小工具。
package winwindow

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	dwmapi                 = syscall.NewLazyDLL("dwmapi.dll")
	procGetWindowLongPtrW  = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW  = user32.NewProc("SetWindowLongPtrW")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procSetWindowRgn       = user32.NewProc("SetWindowRgn")
	procCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	// DwmSetWindowAttribute 在 Windows 10 上对未知属性返回 E_INVALIDARG，
	// 而 dwmapi.dll 本身**一直存在**，所以这里不能靠 LoadDLL 报错判断可用性。
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	// DwmGetWindowAttribute 用来回读设置结果，确认真的生效
	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")
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

const (
	// DWMWA_WINDOW_CORNER_PREFERENCE（Windows 11 起支持）
	dwmwaWindowCornerPreference = 33
	// DWMWCP_ROUND：四角圆角，半径由系统按 DPI 决定
	dwmwcpRound = 2

	// RoundedByDwm / RoundedByRegion 是 SetRoundedCorners 的返回值，
	// 说明实际用了哪条路：前者是系统级圆角（带抗锯齿），后者是裁剪回退。
	RoundedByDwm    = "dwm"
	RoundedByRegion = "region"
)

// winRect 是 Win32 的 RECT。
type winRect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

// SetRoundedCorners 把窗口四角变圆，返回实际生效的方式（见 RoundedByDwm / RoundedByRegion）。
//
// 为什么需要它：Wails 的无边框窗口是**直角矩形**，而界面里画的是圆角卡片。
// 卡片若铺满窗口，窗口的直角就会把卡片「切」成直角（看起来像圆角卡片外面还套了
// 一层直角框）；卡片若留边距，露出来的窗口背景同样是一圈直角。只有让窗口自己
// 变圆，界面上才会真的只有一层圆角。
//
// 优先用 DWM（Windows 11）：带抗锯齿、半径随系统 DPI，视觉最干净。设置完还会
// **回读确认**：有些系统版本会接受调用但不真正生效。DWM 不可用或没生效时退回
// SetWindowRgn 把窗口裁成圆角矩形 —— 边缘会有一点锯齿，但至少不是直角框。
func SetRoundedCorners(handle unsafe.Pointer, radius int) (string, error) {
	hwnd := uintptr(handle)
	if hwnd == 0 {
		return "", fmt.Errorf("窗口句柄为空")
	}

	if err := applyDwmRounding(hwnd); err == nil {
		return RoundedByDwm, nil
	}
	if err := applyRoundedRegion(hwnd, radius); err != nil {
		return "", err
	}
	return RoundedByRegion, nil
}

// applyDwmRounding 让 DWM 给窗口加圆角，并回读确认真的生效。
func applyDwmRounding(hwnd uintptr) error {
	preference := int32(dwmwcpRound)
	result, _, _ := procDwmSetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&preference)),
		unsafe.Sizeof(preference),
	)
	if result != 0 { // 非 S_OK
		return fmt.Errorf("DwmSetWindowAttribute 返回 0x%X", uint32(result))
	}

	var actual int32
	read, _, _ := procDwmGetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&actual)),
		unsafe.Sizeof(actual),
	)
	if read != 0 {
		return fmt.Errorf("回读圆角设置返回 0x%X", uint32(read))
	}
	if actual != dwmwcpRound {
		return fmt.Errorf("圆角设置未生效（实际为 %d）", actual)
	}
	return nil
}

// applyRoundedRegion 用 SetWindowRgn 把窗口裁剪成圆角矩形（DWM 不可用时的退路）。
func applyRoundedRegion(hwnd uintptr, radius int) error {
	if radius <= 0 {
		radius = 16
	}

	var rect winRect
	if ok, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return fmt.Errorf("读取窗口尺寸失败")
	}
	width := int(rect.right - rect.left)
	height := int(rect.bottom - rect.top)
	if width <= 0 || height <= 0 {
		return fmt.Errorf("窗口尺寸还没有就绪")
	}

	// 椭圆直径是半径的两倍；右下角要 +1，否则最右一列/最下一行像素会被裁掉。
	region, _, _ := procCreateRoundRectRgn.Call(
		0, 0,
		uintptr(width+1), uintptr(height+1),
		uintptr(radius*2), uintptr(radius*2),
	)
	if region == 0 {
		return fmt.Errorf("创建圆角区域失败")
	}

	// SetWindowRgn 成功时由系统接管该区域的释放；失败要自己删，否则泄漏 GDI 对象。
	if ok, _, _ := procSetWindowRgn.Call(hwnd, region, 1); ok == 0 {
		_, _, _ = procDeleteObject.Call(region)
		return fmt.Errorf("应用圆角区域失败")
	}
	return nil
}
