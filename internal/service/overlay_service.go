package service

import (
	"context"
	"sync"

	"yukihub/internal/appconf"
	"yukihub/internal/applog"
	"yukihub/internal/common/vo"
	"yukihub/internal/wailsruntime"
)

// OpenFriendsPanelEvent 是「展开好友面板」的前端事件。
//
// 通知浮层被点、或者用户从别处想直接进好友，都发这个事件，由主界面的侧栏
// 监听后打开弹窗 —— 浮层窗口不需要知道好友面板挂在哪个组件上。
const OpenFriendsPanelEvent = "friend:open-panel"

// OverlayService 管理「浮层类窗口」：游戏内好友栏（overlay）与好友通知浮层。
//
// 窗口本身由 main.go 持有（它们必须在主线程创建，且属于 Wails 那一层的
// 细节），这里只负责：快捷键的读写与校验、通知内容的暂存、以及通知被点后
// 「把用户带回好友面板」。窗口操作通过注入的函数回调完成。
type OverlayService struct {
	ctx        context.Context
	config     *appconf.AppConfig
	saveConfig func(*appconf.AppConfig) error
	runtime    wailsruntime.Runtime

	mu sync.Mutex
	// shortcutApplier 由 main 注入：真正去注册全局快捷键，返回最终生效的组合。
	shortcutApplier func(accelerator string) (string, error)
	// activeShortcut 是最近一次注册成功的组合（空 = 没注册上）。
	activeShortcut string
	// noticeHider 由 main 注入：收起通知浮层。
	noticeHider func()
	// overlayToggler 由 main 注入：呼出/收起好友栏（设置页「试一下」按钮用）。
	overlayToggler func()

	// pendingNotice 是当前正等着展示的通知。通知浮层每次显示都是重新创建 /
	// 刚 Show 出来的，前端挂载时机晚于「推送」时机，所以必须能拉一次。
	pendingNotice *FriendPlayEvent
	noticeSeq     int64
}

func NewOverlayService() *OverlayService {
	return &OverlayService{
		runtime:    wailsruntime.Unavailable(),
		saveConfig: appconf.SaveConfig,
	}
}

//wails:ignore
func (s *OverlayService) Init(ctx context.Context, config *appconf.AppConfig) {
	s.ctx = ctx
	s.config = config
}

//wails:ignore
func (s *OverlayService) SetRuntime(runtime wailsruntime.Runtime) {
	if runtime != nil {
		s.runtime = runtime
	}
}

//wails:ignore
func (s *OverlayService) SetShortcutApplier(applier func(accelerator string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shortcutApplier = applier
}

//wails:ignore
func (s *OverlayService) SetNoticeHider(hider func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noticeHider = hider
}

//wails:ignore
func (s *OverlayService) SetActiveShortcut(accelerator string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeShortcut = accelerator
}

// GetOverlayShortcut 返回当前快捷键的配置值与实际生效值。
func (s *OverlayService) GetOverlayShortcut() vo.OverlayShortcut {
	s.mu.Lock()
	configured := s.configuredShortcutLocked()
	active := s.activeShortcut
	s.mu.Unlock()
	return buildOverlayShortcutInfo(configured, active)
}

// SetOverlayShortcut 改快捷键：先真的注册成功，再写进配置。
//
// 顺序不能反 —— 反了的话，注册失败（被别的程序占用）也已经把配置改了，
// 下次启动就会用一个注册不上的组合。
func (s *OverlayService) SetOverlayShortcut(accelerator string) (vo.OverlayShortcut, error) {
	normalized, err := NormalizeOverlayShortcut(accelerator)
	if err != nil {
		return s.GetOverlayShortcut(), err
	}
	return s.applyOverlayShortcut(normalized)
}

// ResetOverlayShortcut 恢复默认快捷键。
func (s *OverlayService) ResetOverlayShortcut() (vo.OverlayShortcut, error) {
	return s.applyOverlayShortcut(DefaultOverlayShortcut)
}

func (s *OverlayService) applyOverlayShortcut(normalized string) (vo.OverlayShortcut, error) {
	s.mu.Lock()
	applier := s.shortcutApplier
	config := s.config
	s.mu.Unlock()

	if applier == nil {
		// 没注入注册器（比如测试环境）：只写配置，不假装注册成功
		if err := s.persistShortcut(normalized); err != nil {
			return s.GetOverlayShortcut(), err
		}
		return s.GetOverlayShortcut(), nil
	}

	active, err := applier(normalized)
	if err != nil {
		return s.GetOverlayShortcut(), err
	}

	s.mu.Lock()
	s.activeShortcut = active
	s.mu.Unlock()

	if config != nil {
		if err := s.persistShortcut(normalized); err != nil {
			return s.GetOverlayShortcut(), err
		}
	}
	return s.GetOverlayShortcut(), nil
}

func (s *OverlayService) persistShortcut(normalized string) error {
	s.mu.Lock()
	config := s.config
	save := s.saveConfig
	s.mu.Unlock()

	if config == nil || save == nil {
		return nil
	}
	config.OverlayShortcut = normalized
	if err := save(config); err != nil {
		applog.LogWarningf(s.ctx, "保存好友栏快捷键失败：%v", err)
		return err
	}
	return nil
}

func (s *OverlayService) configuredShortcutLocked() string {
	if s.config == nil {
		return DefaultOverlayShortcut
	}
	if _, err := NormalizeOverlayShortcut(s.config.OverlayShortcut); err != nil {
		return DefaultOverlayShortcut
	}
	return s.config.OverlayShortcut
}

// ToggleFriendsOverlay 呼出/收起好友栏（供设置页的「试一下」按钮使用）。
func (s *OverlayService) ToggleFriendsOverlay() {
	// 快捷键回调走的是同一条路，这里刻意不做别的事：保持两种入口行为一致。
	s.mu.Lock()
	// 由 main 注入的切换函数挂在快捷键上，这里通过发事件让主界面转发太绕，
	// 所以直接复用注册时用的同一个回调入口。
	toggler := s.overlayToggler
	s.mu.Unlock()
	if toggler != nil {
		toggler()
	}
}

// OpenFriendsPanel 把用户带回好友面板：恢复主窗口 + 让界面展开好友面板。
//
// 通知浮层被点击时调用（对齐 Steam：点通知打开好友列表）。
func (s *OverlayService) OpenFriendsPanel() error {
	s.mu.Lock()
	hider := s.noticeHider
	s.mu.Unlock()
	if hider != nil {
		hider()
	}
	s.runtime.RestoreWindow()
	s.runtime.ShowWindow()
	s.runtime.Emit(OpenFriendsPanelEvent)
	return nil
}

// DismissFriendPlayNotice 收起通知浮层（浮层自己倒计时结束时会调用）。
func (s *OverlayService) DismissFriendPlayNotice() {
	s.mu.Lock()
	hider := s.noticeHider
	s.pendingNotice = nil
	s.mu.Unlock()
	if hider != nil {
		hider()
	}
}

// GetFriendPlayNotice 返回当前待展示的通知；没有则返回 nil。
//
// 通知浮层前端挂载时拉一次：窗口是刚创建/刚 Show 出来的，前端挂载晚于后端
// 推送，只靠事件会漏掉第一条。
func (s *OverlayService) GetFriendPlayNotice() *FriendPlayEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingNotice == nil {
		return nil
	}
	copied := *s.pendingNotice
	return &copied
}

// SetPendingFriendPlayNotice 暂存一条通知并分配序号，供浮层前端拉取/去重。
//
//wails:ignore
func (s *OverlayService) SetPendingFriendPlayNotice(event FriendPlayEvent) FriendPlayEvent {
	s.mu.Lock()
	s.noticeSeq++
	event.Seq = s.noticeSeq
	stored := event
	s.pendingNotice = &stored
	s.mu.Unlock()
	return event
}

//wails:ignore
func (s *OverlayService) SetOverlayToggler(toggler func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overlayToggler = toggler
}
