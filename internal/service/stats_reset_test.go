package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"yukihub/internal/appconf"
	"yukihub/internal/common/enums"
	"yukihub/internal/common/vo"

	_ "github.com/duckdb/duckdb-go/v2"
)

func setupStatsResetAtTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	stmts := []string{
		`CREATE TABLE games (
			id TEXT PRIMARY KEY,
			name TEXT,
			status TEXT DEFAULT 'unplayed',
			company TEXT DEFAULT '',
			summary TEXT DEFAULT '',
			cover_url TEXT DEFAULT '',
			is_nsfw BOOLEAN DEFAULT FALSE,
			playtime_reset_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE play_sessions (
			id TEXT PRIMARY KEY,
			game_id TEXT,
			start_time TIMESTAMPTZ NOT NULL,
			end_time TIMESTAMPTZ,
			duration INTEGER DEFAULT 0,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE game_tags (
			game_id TEXT,
			name TEXT,
			is_spoiler BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE game_progress (
			id TEXT,
			game_id TEXT,
			chapter TEXT DEFAULT '',
			route TEXT DEFAULT '',
			progress_note TEXT DEFAULT '',
			spoiler_boundary TEXT DEFAULT 'none',
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE categories (
			id TEXT PRIMARY KEY,
			name TEXT,
			emoji TEXT DEFAULT '',
			is_system BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE game_categories (
			game_id TEXT,
			category_id TEXT,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func TestStatsRespectPlaytimeResetAt(t *testing.T) {
	db := setupStatsResetAtTestDB(t)
	defer db.Close()

	resetAt := time.Date(2026, time.September, 20, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(
		`INSERT INTO games (id, name, status, playtime_reset_at, created_at, updated_at) VALUES (?, ?, 'playing', ?, ?, ?)`,
		"g1", "清零过的游戏", resetAt, resetAt.Add(-24*time.Hour), resetAt,
	); err != nil {
		t.Fatalf("insert game: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO games (id, name, status, created_at, updated_at) VALUES (?, ?, 'playing', ?, ?)`,
		"g2", "没清零的游戏", resetAt, resetAt,
	); err != nil {
		t.Fatalf("insert game2: %v", err)
	}

	insert := func(id, gameID string, start time.Time, dur int) {
		if _, err := db.Exec(
			`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id, gameID, start, start.Add(time.Duration(dur)*time.Second), dur, start,
		); err != nil {
			t.Fatalf("insert session %s: %v", id, err)
		}
	}
	// g1：清零前 100 秒 × 3，清零后 50 秒 × 2
	for i := 0; i < 3; i++ {
		insert(fmt.Sprintf("g1-old-%d", i), "g1", resetAt.Add(-72*time.Hour+time.Duration(i)*time.Hour), 100)
	}
	for i := 0; i < 2; i++ {
		insert(fmt.Sprintf("g1-new-%d", i), "g1", resetAt.Add(time.Duration(i+1)*time.Hour), 50)
	}
	insert("g2-a", "g2", resetAt.Add(2*time.Hour), 70)

	svc := NewStatsService()
	svc.Init(context.Background(), db, &appconf.AppConfig{})

	gameStats, err := svc.GetGameStats(vo.GameStatsRequest{GameID: "g1", Dimension: enums.All})
	if err != nil {
		t.Fatalf("GetGameStats: %v", err)
	}
	if gameStats.TotalPlayCount != 2 || gameStats.TotalPlayTime != 100 {
		t.Fatalf("单机统计 = count %d / time %d, want 2 / 100（清零前 300 秒不应计入）",
			gameStats.TotalPlayCount, gameStats.TotalPlayTime)
	}

	period, err := svc.GetGlobalPeriodStats(vo.PeriodStatsRequest{Dimension: enums.All})
	if err != nil {
		t.Fatalf("GetGlobalPeriodStats: %v", err)
	}
	// 计入：g1 新 2 条 100 秒 + g2 1 条 70 秒 = 170 秒 / 3 次
	if period.TotalPlayCount != 3 || period.TotalPlayDuration != 170 {
		t.Fatalf("全局统计 = count %d / time %d, want 3 / 170", period.TotalPlayCount, period.TotalPlayDuration)
	}
	for _, item := range period.PlayTimeLeaderboard {
		if item.GameID == "g1" && item.TotalDuration != 100 {
			t.Fatalf("g1 排行时长 = %d, want 100", item.TotalDuration)
		}
	}
	if period.AllSessionsCount != 3 || period.AllSessionsDuration != 170 {
		t.Fatalf("全部会话 = count %d / time %d, want 3 / 170", period.AllSessionsCount, period.AllSessionsDuration)
	}

	// AI 统计（ai_stats_builder）走同一份口径，也必须排掉清零前的会话。
	builder := NewAIStatsBuilder()
	builder.Init(context.Background(), db, &appconf.AppConfig{})
	aiStats, err := builder.Build(enums.All)
	if err != nil {
		t.Fatalf("AI 统计构建失败: %v", err)
	}
	if aiStats.TotalPlayCount != 3 || aiStats.TotalPlayDuration != 170 {
		t.Fatalf("AI 统计 = count %d / time %d, want 3 / 170",
			aiStats.TotalPlayCount, aiStats.TotalPlayDuration)
	}
}
