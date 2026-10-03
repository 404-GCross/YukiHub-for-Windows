package service

import (
	"context"
	"strings"
	"time"

	"yukihub/internal/applog"
	"yukihub/internal/service/yukihubaccount"
)

const (
	// friendPlayPollInterval 是好友「开始玩」的轮询间隔。
	//
	// 手机版 PresenceService.FRIEND_POLL_INTERVAL_MS 就是 15 秒，注释写着
	// 「比 60s 更接近 Steam 体感」。桌面端照搬 —— 再慢就失去「正在玩」的实时感，
	// 再快则白耗服务端。
	friendPlayPollInterval = 15 * time.Second

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
	if !s.friendPlayNotifyEnabled() {
		return
	}

	friends, err := s.ListFriends()
	if err != nil {
		// 轮询失败不打扰用户：令牌过期、断网都会走到这里，下个周期自然重试
		applog.LogWarningf(s.ctx, "YukiHub 账号：好友列表轮询失败（忽略）：%v", err)
		return
	}

	for _, event := range tracker.observe(friends.Friends) {
		applog.LogDebugf(s.ctx, "好友开始游玩：%s - %s", event.Nickname, event.GameTitle)
		s.emitEvent(friendPlayNotifyEvent, event)
	}
}

// friendPlayNotifyEnabled 读「好友开始玩游戏时通知」开关。
func (s *AccountService) friendPlayNotifyEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config == nil || s.config.YukiHubAccountFriendPlayNotify
}

// SetAccountFriendPlayNotify 开关「好友开始玩游戏时通知」。
func (s *AccountService) SetAccountFriendPlayNotify(enabled bool) error {
	s.mu.Lock()
	if s.config != nil {
		s.config.YukiHubAccountFriendPlayNotify = enabled
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
