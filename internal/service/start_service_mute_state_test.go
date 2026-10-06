package service

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"yukihub/internal/appconf"
	"yukihub/internal/models"
	"yukihub/internal/utils/timerutils"
)

// muteCall 记录一次对注入的 setProcessMuted 的调用。
type muteCall struct {
	processID uint32
	muted     bool
}

// fakeMuter 替身：可控返回值，并记录每次调用。
type fakeMuter struct {
	mu    sync.Mutex
	calls []muteCall
	// script 按顺序消费；耗尽后返回默认值 (true, nil)。
	script []scriptedResponse
}

type scriptedResponse struct {
	matched bool
	err     error
}

func (f *fakeMuter) set(processID uint32, muted bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, muteCall{processID: processID, muted: muted})
	if len(f.script) > 0 {
		response := f.script[0]
		f.script = f.script[1:]
		return response.matched, response.err
	}
	return true, nil
}

func (f *fakeMuter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeMuter) last() muteCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return muteCall{}
	}
	return f.calls[len(f.calls)-1]
}

// focusUpdate 构造一次焦点回调。PID 用测试进程自己的 —— IsProcessPresentByPID
// 对它恒为真，这样能走到真正的静音分支而不是「进程已退出」分支。
func focusUpdate(sessionID, gameID string, focused bool) timerutils.FocusUpdate {
	return timerutils.FocusUpdate{
		GameID:    gameID,
		SessionID: sessionID,
		ProcessID: uint32(os.Getpid()),
		IsFocused: focused,
	}
}

// startMutedSession 起一套真实依赖（同手动计时测试），并挂上一个已注册的
// 活跃会话与可注入的静音替身。
func startMutedSession(t *testing.T, config *appconf.AppConfig) (*StartService, *activePlaySession, *fakeMuter) {
	t.Helper()

	startService, _ := setupManualPlaySessionTest(t, config)
	startService.config.MuteGameInBackground = true

	muter := &fakeMuter{}
	startService.setProcessMuted = muter.set

	session := startService.registerActiveSession(
		"mute-state-session", manualSessionTestGameID, time.Now(), models.Game{Name: "静音状态机测试"}, false,
	)
	return startService, session, muter
}

// 回归：游戏从后台回到前台时**必须**解除静音。
//
// 这个分支曾被节流改动弄丢 —— `!shouldMute` 直接 return，没有检查
// audioMuted，导致回前台后 audioMuted 一直是 true，游戏在前台也没声音。
func TestFocusUpdateUnmutesWhenGameReturnsToForeground(t *testing.T) {
	config := &appconf.AppConfig{CurrentMetadataSource: "vndb"}
	startService, session, muter := startMutedSession(t, config)
	defer func() {
		close(session.done)
	}()

	// 后台 → 静音
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, false))
	if got := muter.count(); got != 1 {
		t.Fatalf("后台第一步应调用一次静音，实际 %d 次", got)
	}
	if !session.audioMuted {
		t.Fatal("后台后 audioMuted 应为 true")
	}

	// 回前台 → 必须解除静音
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, true))
	if got := muter.count(); got != 2 {
		t.Fatalf("回前台应再调用一次解除静音，实际累计 %d 次", got)
	}
	if muter.last().muted {
		t.Fatal("回前台那次调用应该是 muted=false")
	}
	if session.audioMuted {
		t.Fatal("回前台后 audioMuted 应为 false")
	}

	// 继续前台 → 无事可做（不再调 COM）
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, true))
	if got := muter.count(); got != 2 {
		t.Fatalf("前台空闲不应再调用，实际累计 %d 次", got)
	}
}

// 后台静音要节流：10 秒内的重复焦点回调不应反复压 COM。
func TestFocusUpdateThrottlesBackgroundRescan(t *testing.T) {
	config := &appconf.AppConfig{CurrentMetadataSource: "vndb"}
	startService, session, muter := startMutedSession(t, config)
	defer func() {
		close(session.done)
	}()

	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, false))
	if got := muter.count(); got != 1 {
		t.Fatalf("第一次后台应调用一次，实际 %d 次", got)
	}
	// 刚静音完，还在节流窗口内
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, false))
	if got := muter.count(); got != 1 {
		t.Fatalf("节流窗口内不应再调用，实际累计 %d 次", got)
	}

	// 把上次扫描时间拨回 11 秒前 → 允许重扫
	session.audioLastMuteScan = time.Now().Add(-11 * time.Second)
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, false))
	if got := muter.count(); got != 2 {
		t.Fatalf("超出节流窗口应允许重扫，实际累计 %d 次", got)
	}
}

// 恢复失败时状态必须保留以便重试。
//
// 两种失败要分开看：
//   - matched=false, err=nil：这次根本没枚举到该 PID 的会话（可能还没创建音频流），
//     状态原样保留，**不**写 audioLastError
//   - matched=true, err!=nil：部分会话改了、另一条报错，audioMuted 保持 true 等重试，
//     并且要记下错误
func TestFocusUpdateKeepsStateWhenRestoreFails(t *testing.T) {
	config := &appconf.AppConfig{CurrentMetadataSource: "vndb"}
	startService, session, muter := startMutedSession(t, config)
	defer func() {
		close(session.done)
	}()

	// 先正常静音
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, false))
	if session.audioPID != uint32(os.Getpid()) {
		t.Fatalf("audioPID = %d, 期望测试进程 PID", session.audioPID)
	}

	// 场景 A：这次枚举不到会话（matched=false, err=nil）
	muter.script = []scriptedResponse{{matched: false, err: nil}}
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, true))
	if !session.audioMuted {
		t.Fatal("没恢复成时 audioMuted 应保持 true 以便重试")
	}
	if !session.audioStateKnown {
		t.Fatal("没恢复成时 audioStateKnown 应保持 true 以便重试")
	}
	if session.audioLastError != "" {
		t.Fatalf("matched=false 且 err=nil 不应记错误，实际 %q", session.audioLastError)
	}

	// 场景 B：部分成功（matched=true 但有报错）—— audioMuted 保持 true 等重试，且要记错误
	muter.script = []scriptedResponse{{matched: true, err: errFakeAudio}}
	startService.handleFocusUpdate(focusUpdate(session.sessionID, session.gameID, true))
	if !session.audioMuted {
		t.Fatal("部分成功时 audioMuted 应保持 true 以便重试")
	}
	if session.audioLastError == "" {
		t.Fatal("部分成功也应记录错误信息")
	}
}

var errFakeAudio = errors.New("fake audio failure")
