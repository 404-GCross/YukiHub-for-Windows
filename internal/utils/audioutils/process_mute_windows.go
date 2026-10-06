//go:build windows

package audioutils

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clsctxAll         = 0x17
	eRender           = 0
	eMultimedia       = 1
	deviceStateActive = 1
)

var (
	ole32                    = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx       = ole32.NewProc("CoInitializeEx")
	procCoUninitialize       = ole32.NewProc("CoUninitialize")
	procCoCreateInstance     = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
	clsidMMDeviceEnumerator  = windows.GUID{Data1: 0xbcde0395, Data2: 0xe52f, Data3: 0x467c, Data4: [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidIMMDeviceEnumerator   = windows.GUID{Data1: 0xa95664d2, Data2: 0x9614, Data3: 0x4f35, Data4: [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	iidIAudioSessionManager2 = windows.GUID{Data1: 0x77aa99a0, Data2: 0x1bd6, Data3: 0x484f, Data4: [8]byte{0x8b, 0xc7, 0x2c, 0x65, 0x4c, 0x9a, 0x9b, 0x6f}}
	iidIAudioSessionControl2 = windows.GUID{Data1: 0xbfb7ff88, Data2: 0x7239, Data3: 0x4fc9, Data4: [8]byte{0x8f, 0xa2, 0x07, 0xc9, 0x50, 0xbe, 0x9c, 0x6d}}
	iidISimpleAudioVolume    = windows.GUID{Data1: 0x87ce5498, Data2: 0x68d6, Data3: 0x44e5, Data4: [8]byte{0x92, 0x15, 0x6d, 0xa4, 0x7e, 0xf8, 0x83, 0xd8}}
)

type iUnknown struct {
	vtbl *iUnknownVtbl
}

type iUnknownVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
}

type iMMDeviceEnumerator struct {
	vtbl *iMMDeviceEnumeratorVtbl
}

type iMMDeviceEnumeratorVtbl struct {
	iUnknownVtbl
	enumAudioEndpoints                     uintptr
	getDefaultAudioEndpoint                uintptr
	getDevice                              uintptr
	registerEndpointNotificationCallback   uintptr
	unregisterEndpointNotificationCallback uintptr
}

type iMMDevice struct {
	vtbl *iMMDeviceVtbl
}

type iMMDeviceCollection struct {
	vtbl *iMMDeviceCollectionVtbl
}

type iMMDeviceCollectionVtbl struct {
	iUnknownVtbl
	getCount uintptr
	item     uintptr
}

type iMMDeviceVtbl struct {
	iUnknownVtbl
	activate          uintptr
	openPropertyStore uintptr
	getID             uintptr
	getState          uintptr
}

type iAudioSessionManager2 struct {
	vtbl *iAudioSessionManager2Vtbl
}

type iAudioSessionManager2Vtbl struct {
	iUnknownVtbl
	getAudioSessionControl        uintptr
	getSimpleAudioVolume          uintptr
	getSessionEnumerator          uintptr
	registerSessionNotification   uintptr
	unregisterSessionNotification uintptr
	registerDuckNotification      uintptr
	unregisterDuckNotification    uintptr
}

type iAudioSessionEnumerator struct {
	vtbl *iAudioSessionEnumeratorVtbl
}

type iAudioSessionEnumeratorVtbl struct {
	iUnknownVtbl
	getCount   uintptr
	getSession uintptr
}

type iAudioSessionControl2 struct {
	vtbl *iAudioSessionControl2Vtbl
}

type iAudioSessionControl2Vtbl struct {
	iUnknownVtbl
	getState                           uintptr
	getDisplayName                     uintptr
	setDisplayName                     uintptr
	getIconPath                        uintptr
	setIconPath                        uintptr
	getGroupingParam                   uintptr
	setGroupingParam                   uintptr
	registerAudioSessionNotification   uintptr
	unregisterAudioSessionNotification uintptr
	getSessionIdentifier               uintptr
	getSessionInstanceIdentifier       uintptr
	getProcessID                       uintptr
	isSystemSoundsSession              uintptr
	setDuckingPreference               uintptr
}

type iSimpleAudioVolume struct {
	vtbl *iSimpleAudioVolumeVtbl
}

type iSimpleAudioVolumeVtbl struct {
	iUnknownVtbl
	setMasterVolume uintptr
	getMasterVolume uintptr
	setMute         uintptr
	getMute         uintptr
}

func IsProcessMuteSupported() bool {
	return true
}

type retainedAudioSession struct {
	control *iAudioSessionControl2
	volume  *iSimpleAudioVolume
	// restoreFailures 统计这条会话连续恢复失败多少次。超过上限就释放掉：
	// 留着既救不回静音，又会挡住后续对该 PID 的恢复（见 setProcessMuted 的路由）。
	restoreFailures int
}

const maxRetainedRestoreFailures = 5

type processMuteState struct {
	sessions map[uint32]map[string]retainedAudioSession
}

type processMuteResult struct {
	matched bool
	err     error
}

type processMuteRequest struct {
	processID uint32
	muted     bool
	result    chan processMuteResult
}

// All retained COM interfaces belong to this worker's MTA. Keeping the thread
// initialized also keeps the apartment alive between focus updates.
var processMuteRequests = sync.OnceValue(func() chan processMuteRequest {
	requests := make(chan processMuteRequest)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		state := processMuteState{sessions: make(map[uint32]map[string]retainedAudioSession)}
		initialized := false
		defer func() {
			if initialized {
				procCoUninitialize.Call()
			}
		}()
		for request := range requests {
			// 这个 goroutine 是唯一的：它一旦死掉，所有 SetProcessMuted 都会卡在
			// 下面的无缓冲 channel 上 forever，而调用方是持着 session.audioMu 调的，
			// 于是 finalizePlaySession 永久阻塞、这次游玩记录永远写不进库。
			// 所以任何 panic 都必须在这里吃掉，并把错误回传给调用方。
			matched, err := func() (matched bool, err error) {
				defer func() {
					if r := recover(); r != nil {
						matched, err = false, fmt.Errorf("audio session worker panic: %v", r)
					}
				}()
				if !initialized {
					hr, _, _ := procCoInitializeEx.Call(0, windows.COINIT_MULTITHREADED)
					if initErr := checkHRESULT(hr, "initialize COM"); initErr != nil {
						return false, initErr
					}
					initialized = true
				}
				return state.setProcessMuted(request.processID, request.muted)
			}()
			request.result <- processMuteResult{matched: matched, err: err}
		}
	}()
	return requests
})

// processMuteTimeout 是等待 COM worker 回包的上限。worker 仍可能因为 native 层的
// 死锁而卡住，没有超时的话调用方（持着 audioMu）会被永久挂住。
const processMuteTimeout = 10 * time.Second

// SetProcessMuted 静音（或解除静音）指定进程在输出设备上的所有音频会话。
//
// 关键点：**每次静音时把对应的 COM 会话对象保留下来**，只有恢复成功才释放。
// 游戏退出后它的音频流与 PID 都会消失，再去按 PID 枚举会一个都找不到，
// 于是 Windows 那层「每应用静音」的持久设置就留在了系统里 —— 表现为
// 「游戏关了，但别的应用/新进程还是没声音」。保留引用后，即便进程已经退出，
// 我们仍然能把那几条会话恢复成不静音。
//
// matched 为 false 表示当前还没枚举到该进程的音频会话（可以稍后重试），
// 整体操作部分成功（部分会话改了、部分报错）时 matched 仍为 true。
func SetProcessMuted(processID uint32, muted bool) (matched bool, err error) {
	if processID == 0 {
		return false, nil
	}
	result := make(chan processMuteResult, 1)
	requests := processMuteRequests()
	select {
	case requests <- processMuteRequest{processID: processID, muted: muted, result: result}:
	case <-time.After(processMuteTimeout):
		return false, fmt.Errorf("timed out queueing audio mute request for PID %d", processID)
	}
	select {
	case response := <-result:
		return response.matched, response.err
	case <-time.After(processMuteTimeout):
		return false, fmt.Errorf("timed out applying audio mute for PID %d", processID)
	}
}

func (s *processMuteState) restoreRetainedSessions(processID uint32) (matched bool, err error) {
	for id, session := range s.sessions[processID] {
		if restoreErr := setAudioVolumeMuted(session.volume, false); restoreErr != nil {
			// range 里的 session 是副本，计数必须显式写回 map，否则永远清不掉。
			session.restoreFailures++
			if session.restoreFailures >= maxRetainedRestoreFailures {
				// 反复恢复不回来的会话多半已经彻底消失（游戏早就退了）。继续留着
				// 只是白占两个 COM 引用，还会永久堵住这个 PID 的后续恢复路径。
				release(unsafe.Pointer(session.volume))
				release(unsafe.Pointer(session.control))
				delete(s.sessions[processID], id)
			} else {
				s.sessions[processID][id] = session
			}
			if err == nil {
				err = restoreErr
			}
			continue // 还没到上限的继续保留，等下一次重试。
		}
		matched = true
		release(unsafe.Pointer(session.volume))
		release(unsafe.Pointer(session.control))
		delete(s.sessions[processID], id)
	}
	if len(s.sessions[processID]) == 0 {
		delete(s.sessions, processID)
	}
	return matched, err
}

func (s *processMuteState) setProcessMuted(processID uint32, muted bool) (matched bool, err error) {
	if !muted && len(s.sessions[processID]) > 0 {
		// 只恢复我们改过的会话，即使默认输出设备已经换了也不受影响。
		return s.restoreRetainedSessions(processID)
	}

	var deviceEnumerator *iMMDeviceEnumerator
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)),
		0,
		clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&deviceEnumerator)),
	)
	if err := checkHRESULT(hr, "create audio device enumerator"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(deviceEnumerator))
	if !muted {
		// 新起来的前台进程可能继承了老版本残留的静音状态，需要主动清掉。
		return s.restoreProcessOnActiveDevices(deviceEnumerator, processID)
	}

	var device *iMMDevice
	hr, _, _ = syscall.SyscallN(
		deviceEnumerator.vtbl.getDefaultAudioEndpoint,
		uintptr(unsafe.Pointer(deviceEnumerator)),
		eRender,
		eMultimedia,
		uintptr(unsafe.Pointer(&device)),
	)
	if err := checkHRESULT(hr, "get default audio endpoint"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(device))
	return s.setDeviceProcessMuted(device, processID, muted)
}

func (s *processMuteState) restoreProcessOnActiveDevices(enumerator *iMMDeviceEnumerator, processID uint32) (matched bool, err error) {
	var devices *iMMDeviceCollection
	hr, _, _ := syscall.SyscallN(
		enumerator.vtbl.enumAudioEndpoints,
		uintptr(unsafe.Pointer(enumerator)),
		eRender,
		deviceStateActive,
		uintptr(unsafe.Pointer(&devices)),
	)
	if err := checkHRESULT(hr, "enumerate output devices for audio restoration"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(devices))
	var count uint32
	hr, _, _ = syscall.SyscallN(devices.vtbl.getCount, uintptr(unsafe.Pointer(devices)), uintptr(unsafe.Pointer(&count)))
	if err := checkHRESULT(hr, "count output devices"); err != nil {
		return false, err
	}
	var firstErr error
	for index := uint32(0); index < count; index++ {
		var device *iMMDevice
		hr, _, _ = syscall.SyscallN(devices.vtbl.item, uintptr(unsafe.Pointer(devices)), uintptr(index), uintptr(unsafe.Pointer(&device)))
		deviceErr := checkHRESULT(hr, "get output device")
		if deviceErr == nil {
			var changed bool
			changed, deviceErr = s.setDeviceProcessMuted(device, processID, false)
			matched = matched || changed
			release(unsafe.Pointer(device))
		}
		if deviceErr != nil && firstErr == nil {
			firstErr = deviceErr
		}
	}
	return matched, firstErr
}

func (s *processMuteState) setDeviceProcessMuted(device *iMMDevice, processID uint32, muted bool) (matched bool, err error) {
	var sessionManager *iAudioSessionManager2
	hr, _, _ := syscall.SyscallN(
		device.vtbl.activate,
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(&iidIAudioSessionManager2)),
		clsctxAll,
		0,
		uintptr(unsafe.Pointer(&sessionManager)),
	)
	if err := checkHRESULT(hr, "activate audio session manager"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(sessionManager))

	var sessionEnumerator *iAudioSessionEnumerator
	hr, _, _ = syscall.SyscallN(
		sessionManager.vtbl.getSessionEnumerator,
		uintptr(unsafe.Pointer(sessionManager)),
		uintptr(unsafe.Pointer(&sessionEnumerator)),
	)
	if err := checkHRESULT(hr, "enumerate audio sessions"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(sessionEnumerator))

	var count int32
	hr, _, _ = syscall.SyscallN(
		sessionEnumerator.vtbl.getCount,
		uintptr(unsafe.Pointer(sessionEnumerator)),
		uintptr(unsafe.Pointer(&count)),
	)
	if err := checkHRESULT(hr, "count audio sessions"); err != nil {
		return false, err
	}

	var firstErr error
	for index := int32(0); index < count; index++ {
		changed, sessionErr := s.setSessionMuted(sessionEnumerator, index, processID, muted)
		if changed {
			matched = true
		}
		if sessionErr != nil && firstErr == nil {
			firstErr = sessionErr
		}
	}
	return matched, firstErr
}

func (s *processMuteState) setSessionMuted(enumerator *iAudioSessionEnumerator, index int32, processID uint32, muted bool) (bool, error) {
	var sessionControl *iUnknown
	hr, _, _ := syscall.SyscallN(
		enumerator.vtbl.getSession,
		uintptr(unsafe.Pointer(enumerator)),
		uintptr(index),
		uintptr(unsafe.Pointer(&sessionControl)),
	)
	if err := checkHRESULT(hr, "get audio session"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(sessionControl))

	var sessionControl2 *iAudioSessionControl2
	if err := queryInterface(unsafe.Pointer(sessionControl), &iidIAudioSessionControl2, unsafe.Pointer(&sessionControl2)); err != nil {
		return false, nil
	}
	retained := false
	defer func() {
		if !retained {
			release(unsafe.Pointer(sessionControl2))
		}
	}()

	var sessionProcessID uint32
	hr, _, _ = syscall.SyscallN(
		sessionControl2.vtbl.getProcessID,
		uintptr(unsafe.Pointer(sessionControl2)),
		uintptr(unsafe.Pointer(&sessionProcessID)),
	)
	if err := checkHRESULT(hr, "get audio session process ID"); err != nil {
		return false, err
	}
	if sessionProcessID != processID {
		return false, nil
	}

	var volume *iSimpleAudioVolume
	if err := queryInterface(unsafe.Pointer(sessionControl), &iidISimpleAudioVolume, unsafe.Pointer(&volume)); err != nil {
		return false, err
	}
	defer func() {
		if !retained {
			release(unsafe.Pointer(volume))
		}
	}()

	var sessionID string
	if muted {
		var id *uint16
		hr, _, _ = syscall.SyscallN(sessionControl2.vtbl.getSessionInstanceIdentifier, uintptr(unsafe.Pointer(sessionControl2)), uintptr(unsafe.Pointer(&id)))
		if err := checkHRESULT(hr, "get audio session instance ID"); err != nil {
			// 拿不到标识就无法在进程退出后找回这条会话，宁可不静音也不要留下残留。
			return false, err
		}
		sessionID = windows.UTF16PtrToString(id)
		procCoTaskMemFree.Call(uintptr(unsafe.Pointer(id)))
	}
	if err := setAudioVolumeMuted(volume, muted); err != nil {
		return false, err
	}
	if muted {
		if s.sessions[processID] == nil {
			s.sessions[processID] = make(map[string]retainedAudioSession)
		}
		if _, exists := s.sessions[processID][sessionID]; !exists {
			s.sessions[processID][sessionID] = retainedAudioSession{control: sessionControl2, volume: volume}
			retained = true
		}
	}
	return true, nil
}

func setAudioVolumeMuted(volume *iSimpleAudioVolume, muted bool) error {
	muteValue := uintptr(0)
	if muted {
		muteValue = 1
	}
	eventContext := windows.GUID{}
	hr, _, _ := syscall.SyscallN(
		volume.vtbl.setMute,
		uintptr(unsafe.Pointer(volume)),
		muteValue,
		uintptr(unsafe.Pointer(&eventContext)),
	)
	return checkHRESULT(hr, "set audio session mute state")
}

func queryInterface(instance unsafe.Pointer, iid *windows.GUID, destination unsafe.Pointer) error {
	unknown := (*iUnknown)(instance)
	hr, _, _ := syscall.SyscallN(
		unknown.vtbl.queryInterface,
		uintptr(instance),
		uintptr(unsafe.Pointer(iid)),
		uintptr(destination),
	)
	return checkHRESULT(hr, "query audio interface")
}

func release(instance unsafe.Pointer) {
	if instance == nil {
		return
	}
	unknown := (*iUnknown)(instance)
	syscall.SyscallN(unknown.vtbl.release, uintptr(instance))
}

func checkHRESULT(hr uintptr, operation string) error {
	if int32(hr) >= 0 {
		return nil
	}
	return fmt.Errorf("%s: HRESULT 0x%08X", operation, uint32(hr))
}
