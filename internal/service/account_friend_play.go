package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"yukihub/internal/applog"
	"yukihub/internal/service/yukihubaccount"
	"yukihub/internal/utils/nativenotify"
)

const (
	// friendPlayPollInterval 是好友状态的轮询间隔。
	//
	// 手机版 PresenceService.FRIEND_POLL_INTERVAL_MS 是 15 秒，但实测下来
	// 「好友都上线了列表还没变、通知更慢」的体感很差 —— 桌面端还多一层：
	// 前端以前自己 30 秒拉一次列表，和后端通知各跑各的，两者不一致。
	// 现在这里 10 秒统一驱动列表推送与通知，比手机版更跟手一点。
	friendPlayPollInterval = 10 * time.Second

	// friendListUpdatedEvent 把好友列表整份推给前端。
	//
	// 为什么要推整份而不是只发个信号让前端自己再拉一次：这样「列表里看到的」
	// 和「通知的依据」永远是同一份快照，不会再出现「列表已经显示在玩、通知
	// 却没来」。同时也省掉前端那路重复请求。
	friendListUpdatedEvent = "friend:list-updated"

	// FriendPlayNoticeEvent 是「好友开始玩游戏」的通知事件名。
	//
	// 它的消费者是全局通知浮层：main.go 把它推给浮层窗口。刻意不做成
	// 「应用内 toast」—— 好友开玩的消息大半发生在用户正泡在游戏里的时候，
	// 只在 YukiHub 窗口里弹等于没提示。
	FriendPlayNoticeEvent = "friend:playing"

	// playingActivityPrefix 是 activity 里「正在玩」的固定前缀。
	//
	// 它是**发给服务端的原文**，与界面语言无关 —— 手机版同样是硬编码中文
	// （PresenceManager.buildPlayingText）。前端剥前缀时必须用这个中文常量，
	// 不能用当前语言的前缀去切，否则英文界面永远剥不掉。
	playingActivityPrefix = "正在玩："
)

// stripPlayingPrefix 去掉 activity 的中文前缀，只留游戏名。
func stripPlayingPrefix(activity string) string {
	trimmed := strings.TrimSpace(activity)
	return strings.TrimSpace(strings.TrimPrefix(trimmed, playingActivityPrefix))
}

// FriendPlayEvent 是一条「好友开始玩游戏」的通知内容。
//
// 前端拿它渲染 Steam 风格卡片（头像 + 「昵称 正在玩 / 绿色游戏名」）。
//
// 昵称做成数组是为了「同一游戏多人同时开始」能合成一条通知，只有一个好友时
// 长度为 1。（注意：这不是照着某张截图的推论，而是为了不刷屏 —— 一次弹出
// 三四条同一个游戏的卡片没有意义。）
type FriendPlayEvent struct {
	UIDs      []int64  `json:"uids"`
	Nicknames []string `json:"nicknames"`
	Avatars   []string `json:"avatars,omitempty"`
	GameTitle string   `json:"game_title"`
	// Seq 是通知序号，由 OverlayService 分配。通知浮层「挂载时拉一次 +
	// 订阅事件」两条路都会拿到同一条，靠它去重。
	Seq int64 `json:"seq,omitempty"`
}

// friendPlayTracker 用「上一次快照」比对出「刚开始玩」和「换了游戏」。
//
// 与手机版 social/FriendNotifier.processFriendsSnapshot 同构：
//   - primed 保证**首轮只建基线不通知** —— 否则每次启动都会把当时所有在玩的
//     好友一次性弹一遍，像刷屏；
//   - 只在好友 online 时认 activity：离线好友的 activity 往往是残留值；
//   - 「停止游玩」不通知（Steam 也不通知），只通知开始玩和换游戏。
type friendPlayTracker struct {
	lastActivity map[int64]string
	primed       bool
}

// observe 吞入新快照，返回需要弹通知的条目。
func (t *friendPlayTracker) observe(friends []yukihubaccount.Friend) []FriendPlayEvent {
	next := make(map[int64]string, len(friends))

	// 按游戏名归组：同一轮里多个好友开始玩同一个游戏时合成一条，
	// 不然会连着弹三张一样的卡（游戏名相同，只是人不同）。
	grouped := make(map[string]*FriendPlayEvent, 2)
	order := make([]string, 0, 2)

	for _, friend := range friends {
		activity := ""
		if friend.Status == yukihubaccount.PresenceOnline {
			activity = strings.TrimSpace(friend.Activity)
		}
		next[friend.UID] = activity

		// 首轮只建基线；「不在玩」不是通知事件
		if !t.primed || activity == "" {
			continue
		}
		if t.lastActivity[friend.UID] == activity {
			continue
		}

		title := stripPlayingPrefix(activity)
		existing, ok := grouped[title]
		if !ok {
			grouped[title] = &FriendPlayEvent{
				UIDs:      []int64{friend.UID},
				Nicknames: []string{friendDisplayName(friend)},
				Avatars:   []string{friend.Avatar},
				GameTitle: title,
			}
			order = append(order, title)
			continue
		}
		existing.UIDs = append(existing.UIDs, friend.UID)
		existing.Nicknames = append(existing.Nicknames, friendDisplayName(friend))
		existing.Avatars = append(existing.Avatars, friend.Avatar)
	}

	events := make([]FriendPlayEvent, 0, len(order))
	for _, title := range order {
		events = append(events, *grouped[title])
	}

	t.lastActivity = next
	t.primed = true
	return events
}

// SetFriendPlayNoticePresenter 注入全局通知浮层的展示器（由 main 装配）。
//
// 返回 false 表示浮层不可用（没有桌面会话 / 建窗失败），调用方退回系统通知。
//
//wails:ignore
func (s *AccountService) SetFriendPlayNoticePresenter(presenter func(FriendPlayEvent) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noticePresenter = presenter
}

// presentFriendPlayNotice 把通知交给全局浮层，浮层不可用返回 false。
func (s *AccountService) presentFriendPlayNotice(event FriendPlayEvent) bool {
	s.mu.Lock()
	presenter := s.noticePresenter
	s.mu.Unlock()
	if presenter == nil {
		return false
	}
	return presenter(event)
}

// friendPlayNoticeText 系统通知的正文。
//
// 系统通知由 Go 侧发出，拿不到前端的界面语言，所以这里固定用中文 ——
// 与手机版 FriendNotifier 的「开始玩 《游戏名》」一致。
func friendPlayNoticeText(event FriendPlayEvent) string {
	return "开始玩 《" + event.GameTitle + "》"
}

// friendPlayNoticeTitle 系统通知的标题：多个好友用顿号连接。
func friendPlayNoticeTitle(event FriendPlayEvent) string {
	if len(event.Nicknames) == 0 {
		return "好友"
	}
	return strings.Join(event.Nicknames, "、")
}

// systemNotifier 惰性创建系统通知器。
//
// 惰性而不是在 Init 里创建：没有桌面会话（CI、服务方式启动）时创建会失败，
// 那时应该静默降级成「只发应用内通知」，而不是让账号服务起不来。
func (s *AccountService) systemNotifier() *nativenotify.Notifier {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()

	if s.notifyReady {
		return s.notifier
	}
	s.notifyReady = true

	notifier, err := nativenotify.New()
	if err != nil {
		applog.LogWarningf(s.ctx, "系统通知不可用，降级为仅应用内通知：%v", err)
		return nil
	}
	s.notifier = notifier
	return notifier
}

// notifyFriendPlayNatively 用系统通知把「好友开始玩游戏」带到用户眼前。
func (s *AccountService) notifyFriendPlayNatively(event FriendPlayEvent) {
	notifier := s.systemNotifier()
	if notifier == nil {
		return
	}
	// 文案与手机版 FriendNotifier 一致：标题=昵称，正文=开始玩 《游戏名》
	body := friendPlayNoticeText(event)
	if err := notifier.Notify(friendPlayNoticeTitle(event), body); err != nil {
		applog.LogWarningf(s.ctx, "发送系统通知失败（忽略）：%v", err)
	}
}

// CloseNativeNotifier 移除系统通知用的隐藏托盘图标。
//
// 应用退出时必须调用：不显式移除的话 Windows 会在托盘区留下「幽灵图标」，
// 直到鼠标划过那一小块才会消失。
func (s *AccountService) CloseNativeNotifier() {
	s.closeSystemNotifier()
}

// closeSystemNotifier 移除隐藏托盘图标。
func (s *AccountService) closeSystemNotifier() {
	s.notifyMu.Lock()
	notifier := s.notifier
	s.notifier = nil
	s.notifyReady = false
	s.notifyMu.Unlock()

	if notifier != nil {
		notifier.Close()
	}
}

// friendListSignature 生成好友列表的轻量签名，用于判断「有没有值得刷新的变化」。
//
// 覆盖用户能直接看到的字段：在线状态、正在玩什么、最后一条消息、未读数、
// 昵称 / 头像 / 备注。刻意不包含会抖动但对界面无意义的字段。
func friendListSignature(list yukihubaccount.FriendList) string {
	var builder strings.Builder
	for _, friend := range list.Friends {
		builder.WriteString(strconv.FormatInt(friend.UID, 10))
		builder.WriteByte('|')
		builder.WriteString(friend.Status)
		builder.WriteByte('|')
		builder.WriteString(friend.Activity)
		builder.WriteByte('|')
		builder.WriteString(friend.LastMessage)
		builder.WriteByte('|')
		builder.WriteString(friend.LastMessageAt)
		builder.WriteByte('|')
		builder.WriteString(strconv.Itoa(friend.UnreadCount))
		builder.WriteByte('|')
		builder.WriteString(friend.Nickname)
		builder.WriteByte('|')
		builder.WriteString(friend.Avatar)
		builder.WriteByte('|')
		builder.WriteString(friend.Note)
		builder.WriteByte('\n')
	}
	builder.WriteString("pending=")
	builder.WriteString(strconv.Itoa(list.PendingCount))
	return builder.String()
}

// friendDisplayName 取展示名：备注优先，与好友列表一致。
func friendDisplayName(friend yukihubaccount.Friend) string {
	if note := strings.TrimSpace(friend.Note); note != "" {
		return note
	}
	if nickname := strings.TrimSpace(friend.Nickname); nickname != "" {
		return nickname
	}
	return "好友"
}

// startFriendPlayPolling 启动「好友开始玩」轮询。
//
// 与 startPresence 一样是登录后才跑的常驻循环；重复调用安全。
func (s *AccountService) startFriendPlayPolling() {
	s.mu.Lock()
	if s.friendPlayCancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(s.resolveContext(nil))
	s.friendPlayCancel = cancel
	s.friendPlayActive = true
	s.mu.Unlock()

	go func() {
		tracker := &friendPlayTracker{}
		// 先立刻跑一轮把基线建起来（这一轮不会弹通知）
		s.pollFriendPlay(tracker)

		ticker := time.NewTicker(friendPlayPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.pollFriendPlay(tracker)
			}
		}
	}()
}

// stopFriendPlayPolling 停止轮询并清掉基线。
func (s *AccountService) stopFriendPlayPolling() {
	s.mu.Lock()
	cancel := s.friendPlayCancel
	s.friendPlayCancel = nil
	s.friendPlayActive = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// pollFriendPlay 拉一次好友列表并派发通知。
func (s *AccountService) pollFriendPlay(tracker *friendPlayTracker) {
	if !s.isLoggedIn() {
		return
	}

	friends, err := s.ListFriends()
	if err != nil {
		// 轮询失败不打扰用户：令牌过期、断网都会走到这里，下个周期自然重试
		applog.LogWarningf(s.ctx, "YukiHub 账号：好友列表轮询失败（忽略）：%v", err)
		return
	}

	// 列表有实质变化才推给前端（在线状态 / 正在玩 / 最后一条消息 / 未读数 /
	// 昵称头像备注，任何一个变了都值得界面刷新一次）。没变化就一声不吭，
	// 免得每 10 秒白推一次。
	signature := friendListSignature(friends)
	s.mu.Lock()
	changed := signature != s.friendListSignature
	if changed {
		s.friendListSignature = signature
	}
	s.mu.Unlock()
	if changed {
		s.emitEvent(friendListUpdatedEvent, friends)
	}

	// 通知才受开关控制：关掉通知**不该**连带停掉好友列表刷新，
	// 列表推送是界面基础数据，与「要不要弹提示」是两件事。
	if !s.friendPlayNotifyEnabled() {
		return
	}
	for _, event := range tracker.observe(friends.Friends) {
		applog.LogDebugf(s.ctx, "好友开始游玩：%s - %s", friendPlayNoticeTitle(event), event.GameTitle)

		// 通知一律走**全局浮层**（屏幕右下角的卡片）：与 Steam 一致 ——
		// 它的游玩通知是全局的，不管你当前在看哪个窗口、是不是在游戏里。
		// 之前做成「应用内 toast、且只在 YukiHub 处于前台时才弹」，
		// 结果就是大部分时候用户根本看不到。
		presented := s.presentFriendPlayNotice(event)
		if !presented {
			// 浮层建不出来（没有桌面会话之类）才退回系统通知：样式差一点，
			// 总比什么都没提示好。
			s.notifyFriendPlayNatively(event)
		}
		// 事件照旧广播出去：浮层之外（主界面等）也可能想自己处理。
		s.emitEvent(FriendPlayNoticeEvent, event)
	}
}

// friendPlayNotifyEnabled 读「好友开始玩游戏时通知」开关。
//
// nil（配置里还没有这个字段）算开启 —— 与手机版
// getBoolean(KEY_FRIEND_PLAY_NOTIFY, true) 的默认值一致。
func (s *AccountService) friendPlayNotifyEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config == nil || s.config.YukiHubAccountFriendPlayNotify == nil {
		return true
	}
	return *s.config.YukiHubAccountFriendPlayNotify
}

// SetAccountFriendPlayNotify 开关「好友开始玩游戏时通知」。
func (s *AccountService) SetAccountFriendPlayNotify(enabled bool) error {
	s.mu.Lock()
	if s.config != nil {
		value := enabled
		s.config.YukiHubAccountFriendPlayNotify = &value
		s.persistConfigLocked()
	}
	s.mu.Unlock()

	if enabled {
		// 打开时重建基线：否则会把「打开前就已经在玩」的好友当成刚开始玩
		s.startFriendPlayPolling()
	} else {
		s.stopFriendPlayPolling()
	}
	s.emitStatus()
	return nil
}
