package gamehelper

import (
	"context"
	"testing"

	"yukihub/internal/common/enums"
	"yukihub/internal/common/vo"
)

func TestQueryGamesPlayTimeSumsAfterReset(t *testing.T) {
	db := setupGameListQueryTest(t)
	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, source_type, source_id, playtime_reset_at,
			cached_at, created_at, updated_at
		) VALUES
			('played', 'Played', 'local', '', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('reset-game', 'Reset Game', 'local', '', TIMESTAMPTZ '2026-01-02 00:00:00', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('never', 'Never Played', 'local', '', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES
			('played-1', 'played', TIMESTAMPTZ '2026-02-01 10:00:00', TIMESTAMPTZ '2026-02-01 11:00:00', 3600),
			('played-2', 'played', TIMESTAMPTZ '2026-02-02 10:00:00', TIMESTAMPTZ '2026-02-02 10:30:00', 1800),
			('reset-old', 'reset-game', TIMESTAMPTZ '2026-01-01 10:00:00', TIMESTAMPTZ '2026-01-01 11:00:00', 3600),
			('reset-edge', 'reset-game', TIMESTAMPTZ '2026-01-01 23:00:00', TIMESTAMPTZ '2026-01-02 00:00:00', 60),
			('reset-new', 'reset-game', TIMESTAMPTZ '2026-01-03 10:00:00', TIMESTAMPTZ '2026-01-03 10:10:00', 600);
	`); err != nil {
		t.Fatalf("insert play time fixtures: %v", err)
	}

	all, err := QueryGamesPlayTime(context.Background(), db, nil)
	if err != nil {
		t.Fatalf("query play time: %v", err)
	}
	if got := all["played"]; got != 5400 {
		t.Errorf("played total = %d, want 5400（两条会话求和）", got)
	}
	// 清零前那条 3600 秒必须排除；边界那条（end_time == reset_at）要保留。
	if got := all["reset-game"]; got != 660 {
		t.Errorf("reset-game total = %d, want 660（排除清零前的 3600，保留边界的 60 与之后的 600）", got)
	}
	if _, ok := all["never"]; ok {
		t.Error("never played 的游戏不应出现在结果里")
	}

	// 限定 id 时只返回这几个游戏的时长。
	scoped, err := QueryGamesPlayTime(context.Background(), db, []string{"reset-game", "never"})
	if err != nil {
		t.Fatalf("query scoped play time: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("scoped result = %v, want 只有 reset-game", scoped)
	}
	if got := scoped["reset-game"]; got != 660 {
		t.Errorf("scoped reset-game total = %d, want 660", got)
	}
}

func TestQueryGameListWithPlayTime(t *testing.T) {
	db := setupGameListQueryTest(t)
	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, source_type, source_id,
			cached_at, created_at, updated_at
		) VALUES
			('pt-a', 'Play Time A', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('pt-b', 'Play Time B', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('pt-c', 'Play Time C', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES
			('pt-a-1', 'pt-a', TIMESTAMPTZ '2026-02-01 10:00:00', TIMESTAMPTZ '2026-02-01 11:00:00', 3600),
			('pt-b-1', 'pt-b', TIMESTAMPTZ '2026-02-01 10:00:00', TIMESTAMPTZ '2026-02-01 10:05:00', 300);
	`); err != nil {
		t.Fatalf("insert fixtures: %v", err)
	}

	withPlayTime, err := QueryGameList(context.Background(), db, vo.GameListRequest{
		Limit:        100,
		SearchQuery:  "Play Time",
		SortBy:       enums.GameListSortByName,
		SortOrder:    enums.SortOrderAsc,
		WithPlayTime: true,
	}, GameListScope{})
	if err != nil {
		t.Fatalf("query games with play time: %v", err)
	}
	if len(withPlayTime.Games) != 3 {
		t.Fatalf("games = %d, want 3", len(withPlayTime.Games))
	}
	// 本页每条游戏都要有条目，没玩过的为 0，调用方无需再单查。
	for _, id := range []string{"pt-a", "pt-b", "pt-c"} {
		if _, ok := withPlayTime.PlayTimes[id]; !ok {
			t.Fatalf("play_times 缺少 %s: %v", id, withPlayTime.PlayTimes)
		}
	}
	if got := withPlayTime.PlayTimes["pt-a"]; got != 3600 {
		t.Errorf("pt-a play time = %d, want 3600", got)
	}
	if got := withPlayTime.PlayTimes["pt-c"]; got != 0 {
		t.Errorf("pt-c play time = %d, want 0", got)
	}

	without, err := QueryGameList(context.Background(), db, vo.GameListRequest{
		Limit:       100,
		SearchQuery: "Play Time",
		SortBy:      enums.GameListSortByName,
		SortOrder:   enums.SortOrderAsc,
	}, GameListScope{})
	if err != nil {
		t.Fatalf("query games without play time: %v", err)
	}
	if without.PlayTimes != nil {
		t.Errorf("未请求时不应返回 play_times: %v", without.PlayTimes)
	}
}

func TestQueryGameListSortsByPlayTime(t *testing.T) {
	db := setupGameListQueryTest(t)
	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, source_type, source_id,
			cached_at, created_at, updated_at
		) VALUES
			('sort-mid', 'Sort Mid', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('sort-high', 'Sort High', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('sort-none', 'Sort None', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES
			('sort-mid-1', 'sort-mid', TIMESTAMPTZ '2026-02-01 10:00:00', TIMESTAMPTZ '2026-02-01 11:00:00', 3600),
			('sort-high-1', 'sort-high', TIMESTAMPTZ '2026-02-01 10:00:00', TIMESTAMPTZ '2026-02-01 20:00:00', 36000);
	`); err != nil {
		t.Fatalf("insert fixtures: %v", err)
	}

	response, err := QueryGameList(context.Background(), db, vo.GameListRequest{
		Limit:       100,
		SearchQuery: "Sort",
		SortBy:      enums.GameListSortByPlayTime,
		SortOrder:   enums.SortOrderDesc,
	}, GameListScope{})
	if err != nil {
		t.Fatalf("query games sorted by play time: %v", err)
	}

	got := make([]string, 0, len(response.Games))
	for _, game := range response.Games {
		got = append(got, game.ID)
	}
	want := []string{"sort-high", "sort-mid", "sort-none"}
	if len(got) != len(want) {
		t.Fatalf("game IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("game IDs = %v, want %v", got, want)
		}
	}
	// 排序用的 JOIN 是 1:1，不得放大总数。
	if response.Total != 3 {
		t.Errorf("total = %d, want 3（排序 JOIN 不应改变计数）", response.Total)
	}
}
