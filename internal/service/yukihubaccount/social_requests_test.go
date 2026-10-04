package yukihubaccount

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// 好友申请。
//
// 契约（对齐手机版 SocialApiClient）：
//   - `/friends/list` 的 `pendingRequests` 是**数字**（待处理条数），不是数组；
//   - 申请内容在 `/friends/requests`，返回 `{incoming:[...], outgoing:[...]}`；
//   - 条目里的 `friendshipId` 是**数字**，收到的用 `fromUid`、发出的用 `toUid`；
//   - 接受 / 拒绝按**数字** friendshipId 发回去。
//
// 这条链以前整条是断的：list 里的数字被当数组解析（解析不出东西），
// 又没有 /friends/requests 这个接口，于是「手机上明明有人加我，PC 一片空白」。

func TestListFriendRequestsParsesIncomingAndOutgoing(t *testing.T) {
	const body = `{
		"incoming": [
			{"friendshipId": 41, "fromUid": 7, "nickname": "小夜", "avatarUrl": "/uploads/a.png", "signature": "你好", "createdAt": "2026-10-04T10:00:00Z"}
		],
		"outgoing": [
			{"friendshipId": 42, "toUid": 9, "nickname": "幽raf"}
		]
	}`

	server, requests, _ := newRecordingServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Cloudflare 要求带正常 UA 与站点 Referer（见 client 的公共头）
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Referer") == "" {
			t.Errorf("缺少公共请求头：UA=%q Referer=%q",
				r.Header.Get("User-Agent"), r.Header.Get("Referer"))
		}
		writeJSON(w, http.StatusOK, body)
	})

	client := NewClient(server.URL)
	result, err := client.ListFriendRequests(context.Background(), "token")
	if err != nil {
		t.Fatalf("ListFriendRequests: %v", err)
	}

	if len(result.Incoming) != 1 || len(result.Outgoing) != 1 {
		t.Fatalf("收到的/发出的条数不对：%d / %d", len(result.Incoming), len(result.Outgoing))
	}

	incoming := result.Incoming[0]
	if incoming.FriendshipID != 41 {
		t.Errorf("friendshipId 解析失败（数字字段），实际 %d", incoming.FriendshipID)
	}
	if incoming.UID != 7 {
		t.Errorf("收到的申请应取 fromUid，实际 %d", incoming.UID)
	}
	if incoming.Nickname != "小夜" || incoming.Avatar != "/uploads/a.png" {
		t.Errorf("昵称/头像解析不对：%+v", incoming)
	}
	if incoming.Outgoing {
		t.Error("收到的申请不该被标成 outgoing")
	}

	outgoing := result.Outgoing[0]
	if outgoing.UID != 9 {
		t.Errorf("发出的申请应取 toUid，实际 %d", outgoing.UID)
	}
	if !outgoing.Outgoing {
		t.Error("发出的申请必须标成 outgoing（界面要分两段渲染）")
	}

	if len(*requests) != 1 {
		t.Fatalf("应只发一次请求，实际 %d", len(*requests))
	}
	last := (*requests)[0]
	if last.Method != http.MethodGet || last.Path != "/friends/requests" {
		t.Errorf("请求不对：%s %s", last.Method, last.Path)
	}
	// 拉取是 GET（没有请求体），不该带任何敏感值到 query 里
	if last.RawQuery != "" {
		t.Errorf("查询串应为空，实际 %q", last.RawQuery)
	}
}

// 头像字段名在不同版本下不一样（avatarUrl / avatar_url / avatar），逐个兜住。
func TestListFriendRequestsAvatarAliases(t *testing.T) {
	const body = `{"incoming":[
		{"friendshipId":1,"fromUid":1,"avatarUrl":"/a.png"},
		{"friendshipId":2,"fromUid":2,"avatar_url":"/b.png"},
		{"friendshipId":3,"fromUid":3,"avatar":"/c.png"}
	]}`

	server, _, _ := newRecordingServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, body)
	})

	client := NewClient(server.URL)
	result, err := client.ListFriendRequests(context.Background(), "token")
	if err != nil {
		t.Fatalf("ListFriendRequests: %v", err)
	}

	want := []string{"/a.png", "/b.png", "/c.png"}
	if len(result.Incoming) != len(want) {
		t.Fatalf("条数不对：%d", len(result.Incoming))
	}
	for index, expected := range want {
		if result.Incoming[index].Avatar != expected {
			t.Errorf("第 %d 条头像 = %q，期望 %q", index, result.Incoming[index].Avatar, expected)
		}
	}
}

// 列表接口里的 `pendingRequests` 是数字（待处理条数），必须被当成计数读出来。
// 以前只认 pendingCount/totalPending/requestCount，于是角标恒为 0。
func TestListFriendsReadsPendingRequestsAsCount(t *testing.T) {
	const body = `{"friends":[],"pendingRequests":3}`

	server, _, _ := newRecordingServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, body)
	})

	client := NewClient(server.URL)
	list, err := client.ListFriends(context.Background(), "token")
	if err != nil {
		t.Fatalf("ListFriends: %v", err)
	}
	if list.PendingCount != 3 {
		t.Errorf("PendingCount = %d，期望 3（pendingRequests 是数字）", list.PendingCount)
	}
	if len(list.PendingRequests) != 0 {
		t.Errorf("数字不该被解析成申请数组，实际 %d 条", len(list.PendingRequests))
	}
}

// 接受 / 拒绝必须把 friendshipId 当**数字**发出去：服务端与手机版都是数字，
// 发字符串会被类型校验拒掉（点了没反应）。
func TestAcceptAndRejectSendNumericFriendshipID(t *testing.T) {
	server, requests, _ := newRecordingServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"success":true}`)
	})

	client := NewClient(server.URL)
	if err := client.AcceptFriendRequest(context.Background(), "token", 41, 0); err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
	if err := client.RejectFriendRequest(context.Background(), "token", 42); err != nil {
		t.Fatalf("RejectFriendRequest: %v", err)
	}

	if len(*requests) != 2 {
		t.Fatalf("应有两次请求，实际 %d", len(*requests))
	}

	for index, expectedPath := range []string{"/friends/accept", "/friends/reject"} {
		request := (*requests)[index]
		if request.Method != http.MethodPost || request.Path != expectedPath {
			t.Errorf("请求不对：%s %s", request.Method, request.Path)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(request.Body), &payload); err != nil {
			t.Fatalf("解析请求体失败：%v（%s）", err, request.Body)
		}
		if _, ok := payload["friendshipId"].(float64); !ok {
			t.Errorf("%s 的 friendshipId 必须是数字，实际 %T（%v）",
				expectedPath, payload["friendshipId"], payload["friendshipId"])
		}
	}

	// 只有 uid 的场景（资料页接受申请）仍要发得出 uid
	if err := client.AcceptFriendRequest(context.Background(), "token", 0, 7); err != nil {
		t.Fatalf("AcceptFriendRequest(by uid): %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte((*requests)[2].Body), &payload); err != nil {
		t.Fatalf("解析请求体失败：%v", err)
	}
	if _, ok := payload["uid"].(float64); !ok {
		t.Errorf("按 uid 接受时应带上 uid，实际 %v", payload)
	}
	if _, hasFriendship := payload["friendshipId"]; hasFriendship {
		t.Errorf("同时按 uid 接受时不该塞 friendshipId=0，实际 %v", payload)
	}
}
