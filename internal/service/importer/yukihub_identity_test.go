package importer

import (
	"fmt"
	"testing"
	"time"
	"yukihub/internal/models/yukihub"
)

// 桌面端自己扫描进来的游戏没有 Android 侧的整数 ID，快照里 local_id 恒为 0。
// 若按 local_id 归组，所有这类游戏会挤进同一个桶 —— 备份回导 / 云同步合并时，
// 每个游戏都会拿到**全部游戏**的元数据与游玩记录。
// 这里钉住「local_id 为 0 时按标题归组」的行为（对齐手机版的三档匹配）。
func TestYukiHubSnapshotIndexFallsBackToTitle(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.October, 5, 20, 0, 0, 0, time.Local)
	sessions := []yukihub.PlaySession{
		{SessionUUID: "uuid-a", GameLocalID: 0, GameTitle: "游戏甲", StartTime: start.UnixMilli(), EndTime: start.Add(time.Hour).UnixMilli(), Duration: 3_600_000},
		{SessionUUID: "uuid-b", GameLocalID: 0, GameTitle: "游戏乙", StartTime: start.UnixMilli(), EndTime: start.Add(2 * time.Hour).UnixMilli(), Duration: 7_200_000},
		// 来自手机端的条目有真实 local_id，按 local_id 归组，不参与标题索引。
		{SessionUUID: "uuid-c", GameLocalID: 42, GameTitle: "手机端游戏", StartTime: start.UnixMilli(), Duration: 60_000},
	}

	index := indexYukiHubSessions(sessions)

	if got := index.lookup(0, "游戏甲"); len(got) != 1 || got[0].SessionUUID != "uuid-a" {
		t.Fatalf("按标题取「游戏甲」= %+v, want 只有 uuid-a", got)
	}
	if got := index.lookup(0, "游戏乙"); len(got) != 1 || got[0].SessionUUID != "uuid-b" {
		t.Fatalf("按标题取「游戏乙」= %+v, want 只有 uuid-b", got)
	}
	// 标题归一化：首尾空白与大小写差异不应影响匹配。
	if got := index.lookup(0, " 游戏甲 "); len(got) != 1 {
		t.Fatalf("标题归一化后应仍能命中，得到 %d 条", len(got))
	}
	// 有真实 local_id 的条目走 local_id 索引，标题索引里不应出现。
	if got := index.lookup(0, "手机端游戏"); len(got) != 0 {
		t.Fatalf("local_id>0 的条目不应进标题索引，得到 %+v", got)
	}
	if got := index.lookup(42, ""); len(got) != 1 || got[0].SessionUUID != "uuid-c" {
		t.Fatalf("按 local_id 42 取 = %+v, want uuid-c", got)
	}
	// 没有标题、也没有 local_id 的条目落不了桶，只能丢弃（与手机版一致）。
	if got := index.lookup(0, ""); len(got) != 0 {
		t.Fatalf("空标题不应命中任何条目，得到 %+v", got)
	}
}

// 回归：两台 PC 自建游戏的会话必须各归各的。
// 旧实现在这里会把 uuid-a 与 uuid-b 同时塞给「游戏甲」（因为都以 local_id 0 归组），
// 于是每个游戏的时长都变成全部游戏的合计。
func TestConvertYukiHubSessionsKeepsDesktopGamesSeparated(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.October, 5, 20, 0, 0, 0, time.Local)
	index := indexYukiHubSessions([]yukihub.PlaySession{
		{SessionUUID: "uuid-a", GameTitle: "游戏甲", StartTime: start.UnixMilli(), EndTime: start.Add(time.Hour).UnixMilli(), Duration: 3_600_000},
		{SessionUUID: "uuid-b", GameTitle: "游戏乙", StartTime: start.UnixMilli(), EndTime: start.Add(2 * time.Hour).UnixMilli(), Duration: 7_200_000},
	})

	gameA := yukihub.Game{Title: "游戏甲", TotalPlayTime: 3_600_000}
	gameB := yukihub.Game{Title: "游戏乙", TotalPlayTime: 7_200_000}

	gotA := convertYukiHubSessions("game-a", gameA, index.lookup(0, gameA.Title), time.Time{}, time.Time{})
	if len(gotA) != 1 || gotA[0].Duration != 3600 {
		t.Fatalf("游戏甲的会话 = %+v, want 仅 1 条 3600 秒", gotA)
	}
	gotB := convertYukiHubSessions("game-b", gameB, index.lookup(0, gameB.Title), time.Time{}, time.Time{})
	if len(gotB) != 1 || gotB[0].Duration != 7200 {
		t.Fatalf("游戏乙的会话 = %+v, want 仅 1 条 7200 秒", gotB)
	}
	if gotA[0].ID == gotB[0].ID {
		t.Fatalf("两台游戏的会话 ID 撞了：%s", gotA[0].ID)
	}
}

// 确定性 UUID 的种子：local_id>0 时必须与历史实现逐字一致（否则已导入过的备份
// 再导一次会算出新 UUID，旧记录变孤儿、时长被重复累计）；
// local_id==0 时必须带上标题，否则不同游戏会算出同一个 UUID 撞主键。
func TestYukiHubUUIDSeedsStayStableAndDistinct(t *testing.T) {
	t.Parallel()

	mobileGame := yukihub.Game{LocalID: 42, Title: "手机端游戏"}
	if got := yukiHubAggregateUUIDSeed(mobileGame); got != "yukihub:42:aggregate" {
		t.Fatalf("local_id>0 的聚合种子 = %q, want 保持历史格式", got)
	}
	entry := yukihub.PlaySession{StartTime: 1000, EndTime: 2000}
	if got := yukiHubSessionUUIDSeed(mobileGame, entry, 3); got != "yukihub:42:1000:2000:3" {
		t.Fatalf("local_id>0 的会话种子 = %q, want 保持历史格式", got)
	}

	selfA := yukihub.Game{Title: "游戏甲"}
	selfB := yukihub.Game{Title: "游戏乙"}
	if yukiHubAggregateUUIDSeed(selfA) == yukiHubAggregateUUIDSeed(selfB) {
		t.Fatal("两台自建游戏的聚合种子不能相同（会撞 play_sessions 主键）")
	}
	if yukiHubSessionUUIDSeed(selfA, entry, 3) == yukiHubSessionUUIDSeed(selfB, entry, 3) {
		t.Fatal("两台自建游戏同起止时刻的会话种子不能相同")
	}
	// 同一台游戏重复导入必须稳定，命中去重。
	if yukiHubAggregateUUIDSeed(selfA) != yukiHubAggregateUUIDSeed(yukihub.Game{Title: " 游戏甲 "}) {
		t.Fatal("同一台游戏的聚合种子应稳定（标题归一化后一致）")
	}
	want := fmt.Sprintf("yukihub:0:%s:aggregate", "游戏甲")
	if got := yukiHubAggregateUUIDSeed(selfA); got != want {
		t.Fatalf("自建游戏聚合种子 = %q, want %q", got, want)
	}
}
