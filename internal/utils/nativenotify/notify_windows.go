//go:build windows

// Package nativenotify 用 Win32 托盘气泡发系统通知。
//
// 为什么不引第三方库：本机/CI 是离线构建（GOPROXY=off），加不了新依赖；
// 而且 Wails v3 自己也不提供 Windows 原生通知（只有 Android 的 Notify）。
//
// 为什么值得做：应用内 toast 只在 YukiHub 窗口可见时才有意义，
// **全屏游戏时用户根本看不到**。系统通知由系统层绘制，能盖在独占全屏游戏上，
// 这是「好友开始玩游戏」这类提醒在游戏里唯一能到达用户的通道。
//
// 实现方式：建一个 message-only window 承载托盘图标，再用
// Shell_NotifyIcon(NIM_MODIFY, NIF_INFO) 发气泡。图标本身保持隐藏
// （NIS_HIDDEN），用户不点开托盘溢出区就看不到它。
package nativenotify

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
)

const (
	// Shell_NotifyIcon 的动作
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	// NOTIFYICONDATA.uFlags
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifState   = 0x00000008
	nifInfo    = 0x00000010

	// NOTIFYICONDATA.dwState
	nisHidden = 0x00000001

	// NOTIFYICONDATA.dwInfoFlags
	niifInfo    = 0x00000001
	niifWarning = 0x00000002
	niifError   = 0x00000003

	// NOTIFYICONDATA.uCallbackMessage 用的自定义消息号
	notifyCallbackMessage = 0x0400 + 1 // WM_APP + 1

	// IDI_APPLICATION：系统默认图标，避免依赖自己的资源
	idiApplication = 32512

	// HWND_MESSAGE = (HWND)-3
	hwndMessage = ^uintptr(2)

	// windowClass 名（注册一次的全局窗口类）
	windowClass = "YukiHubNativeNotifyWindow"
)

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// notifyIconDataW 对应 Win32 的 NOTIFYICONDATAW。
//
// 字段顺序与对齐必须与 SDK 一致：cbSize 之后紧跟的 UINT 组，
// 再往后每个 HANDLE/指针都按 8 字节对齐，Go 的自然对齐与 C 相同。
type notifyIconDataW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         syscall.GUID
	HBalloonIcon     uintptr
}

type msgW struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// wndProc 是托盘窗口的消息处理。这里只做默认处理 —— 我们不关心托盘图标的
// 鼠标事件（图标本身是隐藏的），但必须要有一个合法的 WndProc 让窗口能创建。
var wndProc = syscall.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
})

var registerClassOnce sync.Once
var registerClassErr error

// Notifier 是一个隐藏托盘图标 + 气泡通知的发送器。
type Notifier struct {
	mu     sync.Mutex
	hwnd   uintptr
	added  bool
	closed bool
}

// New 创建通知器。失败时返回错误，调用方应降级为「只发应用内通知」。
func New() (*Notifier, error) {
	if err := ensureWindowClass(); err != nil {
		return nil, err
	}

	className, _ := syscall.UTF16PtrFromString(windowClass)
	hInstance, _, _ := procGetModuleHandleW.Call(0)

	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0, 0, 0, 0, 0,
		hwndMessage,
		0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("创建通知窗口失败: %v", callErr)
	}

	notifier := &Notifier{hwnd: hwnd}

	data := notifier.baseData()
	data.UFlags = nifIcon | nifTip | nifMessage | nifState
	data.HIcon = loadAppIcon()
	data.DwState = nisHidden
	data.DwStateMask = nisHidden
	copyUTF16(data.SzTip[:], "YukiHub")

	if !shellNotifyIcon(nimAdd, &data) {
		procDestroyWindow.Call(hwnd)
		return nil, fmt.Errorf("注册托盘图标失败")
	}
	notifier.added = true

	// 起一个消息循环：托盘图标的消息会投递到这个窗口，没人取会一直堆在队列里。
	go notifier.messageLoop()

	return notifier, nil
}

// Notify 弹一条系统通知。
func (n *Notifier) Notify(title, message string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed || !n.added {
		return fmt.Errorf("通知器已关闭")
	}

	data := n.baseData()
	data.UFlags = nifInfo
	copyUTF16(data.SzInfoTitle[:], title)
	copyUTF16(data.SzInfo[:], message)
	data.DwInfoFlags = niifInfo

	if !shellNotifyIcon(nimModify, &data) {
		return fmt.Errorf("发送系统通知失败")
	}
	return nil
}

// Close 移除托盘图标并销毁窗口。应用退出时调用。
func (n *Notifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return
	}
	n.closed = true

	if n.added {
		data := n.baseData()
		shellNotifyIcon(nimDelete, &data)
		n.added = false
	}
	if n.hwnd != 0 {
		procDestroyWindow.Call(n.hwnd)
		n.hwnd = 0
	}
}

// messageLoop 抽取托盘窗口的消息，避免队列堆积。
func (n *Notifier) messageLoop() {
	// GetMessage 绑定线程，必须锁在这个 goroutine 上
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var message msgW
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&message)), 0, 0, 0,
		)
		// 0 = WM_QUIT，-1 = 出错
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (n *Notifier) baseData() notifyIconDataW {
	data := notifyIconDataW{
		HWnd:             n.hwnd,
		UID:              1,
		UCallbackMessage: notifyCallbackMessage,
	}
	data.CbSize = uint32(unsafe.Sizeof(data))
	return data
}

func ensureWindowClass() error {
	registerClassOnce.Do(func() {
		className, _ := syscall.UTF16PtrFromString(windowClass)
		hInstance, _, _ := procGetModuleHandleW.Call(0)

		class := wndClassExW{
			LpfnWndProc:   wndProc,
			HInstance:     hInstance,
			LpszClassName: className,
		}
		class.CbSize = uint32(unsafe.Sizeof(class))

		ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if ret == 0 {
			// 类已存在（ERROR_CLASS_ALREADY_EXISTS）不算失败
			const errorClassAlreadyExists = 1410
			if errno, ok := err.(syscall.Errno); ok && errno == errorClassAlreadyExists {
				return
			}
			registerClassErr = fmt.Errorf("注册通知窗口类失败: %v", err)
		}
	})
	return registerClassErr
}

func loadAppIcon() uintptr {
	icon, _, _ := procLoadIconW.Call(0, uintptr(idiApplication))
	return icon
}

func shellNotifyIcon(action uint32, data *notifyIconDataW) bool {
	ret, _, _ := procShellNotifyIconW.Call(
		uintptr(action), uintptr(unsafe.Pointer(data)),
	)
	return ret != 0
}

func copyUTF16(dst []uint16, value string) {
	if len(dst) == 0 {
		return
	}
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		dst[0] = 0
		return
	}
	// copy 自动截断，且 encoded 自带结尾 0
	copy(dst, encoded)
	dst[len(dst)-1] = 0
}
