package service

import (
	"testing"

	"yukihub/internal/service/yukihubaccount"
)

// 「好友开始玩游戏」通知的差异检测。
//
// 对齐手机版 social/FriendNotifier.processFriendsSnapshot：首轮只建基线，
// 只在「不在玩 → 在玩」和「换游戏」时通知，停止游玩不通知。

func testFriend(uid int64, nickname, note, status, activity string) yukihubaccount.Friend {
	return yukihubaccount.Friend{
		UID:      uid,
		Nickname: nickname,
		Note:     note,
		Status:   status,
		Activity: activity,
	}
}

func TestFriendPlayTrackerFirstSnapshotDoesNotNotify(t *testing.T) {
	tracker := &friendPlayTracker{}

	events := tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOnline, "正在玩：樱之刻"),
	})

	if len(events) != 0 {
		t.Fatalf("首轮只建基线，不该弹通知（否则每次启动都刷屏），实际 %d 条", len(events))
	}
}

func TestFriendPlayTrackerNotifiesOnStartAndGameChange(t *testing.T) {
	tracker := &friendPlayTracker{}
	// 基线：没人在玩
	tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOnline, ""),
	})

	// 开始玩 → 通知，且游戏名要剥掉「正在玩：」前缀
	events := tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOnline, "正在玩：樱之刻"),
	})
	if len(events) != 1 {
		t.Fatalf("开始游玩应通知一次，实际 %d 条", len(events))
	}
	if events[0].GameTitle != "樱之刻" {
		t.Errorf("游戏名应剥掉中文前缀，实际 %q", events[0].GameTitle)
	}
	if events[0].Nickname != "Yuki" || events[0].UID != 1 {
		t.Errorf("昵称/UID 不对：%+v", events[0])
	}

	// 换游戏 → 通知
	events = tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOnline, "正在玩：CLANNAD"),
	})
	if len(events) != 1 || events[0].GameTitle != "CLANNAD" {
		t.Fatalf("换游戏应通知一次且游戏名更新，实际 %+v", events)
	}

	// 停止游玩 → 不通知（Steam 也不通知「不玩了」）
	events = tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOnline, ""),
	})
	if len(events) != 0 {
		t.Fatalf("停止游玩不该通知，实际 %d 条", len(events))
	}

	// 同一状态重复出现 → 不重复通知
	tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOnline, "正在玩：樱之刻"),
	})
	events = tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOnline, "正在玩：樱之刻"),
	})
	if len(events) != 0 {
		t.Fatalf("状态没变不该重复通知，实际 %d 条", len(events))
	}
}

func TestFriendPlayTrackerIgnoresOfflineActivity(t *testing.T) {
	tracker := &friendPlayTracker{}
	tracker.observe(nil)

	events := tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "", yukihubaccount.PresenceOffline, "正在玩：樱之刻"),
	})

	if len(events) != 0 {
		t.Fatal("离线好友的 activity 是心跳残留，不该当成「正在玩」通知")
	}
}

func TestFriendPlayTrackerPrefersNote(t *testing.T) {
	tracker := &friendPlayTracker{}
	tracker.observe(nil)

	events := tracker.observe([]yukihubaccount.Friend{
		testFriend(1, "Yuki", "小由纪", yukihubaccount.PresenceOnline, "正在玩：樱之刻"),
	})

	if len(events) != 1 {
		t.Fatalf("应通知一次，实际 %d 条", len(events))
	}
	if events[0].Nickname != "小由纪" {
		t.Errorf("展示名应优先用备注（与好友列表一致），实际 %q", events[0].Nickname)
	}
}

func TestFriendPlayTrackerBackfillsNicknameFallback(t *testing.T) {
	tracker := &friendPlayTracker{}
	tracker.observe(nil)

	// 既没有备注也没有昵称时不能给前端一个空标题
	events := tracker.observe([]yukihubaccount.Friend{
		testFriend(9, "", "", yukihubaccount.PresenceOnline, "正在玩：樱之刻"),
	})
	if len(events) != 1 {
		t.Fatalf("应通知一次，实际 %d 条", len(events))
	}
	if events[0].Nickname == "" {
		t.Error("昵称兜底不应为空")
	}
}
