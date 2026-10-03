package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"yukihub/internal/applog"
	"yukihub/internal/service/yukihubaccount"
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

	// friendPlayNotifyEvent 是后端 → 前端的「好友开始玩游戏」通知事件。
	friendPlayNotifyEvent = "friend:playing"

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
// 前端拿它渲染 Steam 风格弹窗：头像 + 「昵称 开始玩 《游戏名》」。
type FriendPlayEvent struct {
	UID       int64  `json:"uid"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar,omitempty"`
	GameTitle string `json:"game_title"`
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
	events := make([]FriendPlayEvent, 0, 2)

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

		events = append(events, FriendPlayEvent{
			UID:       friend.UID,
			Nickname:  friendDisplayName(friend),
			Avatar:    friend.Avatar,
			GameTitle: stripPlayingPrefix(activity),
		})
	}

	t.lastActivity = next
	t.primed = true
	return events
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
		applog.LogDebugf(s.ctx, "好友开始游玩：%s - %s", event.Nickname, event.GameTitle)
		s.emitEvent(friendPlayNotifyEvent, event)
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
