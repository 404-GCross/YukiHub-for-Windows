package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"yukihub/internal/appconf"
	"yukihub/internal/applog"
	"yukihub/internal/migrations"

	_ "github.com/duckdb/duckdb-go/v2"
)

const manualSessionTestGameID = "manual-session-game"

// setupManualPlaySessionTest 起一套真实依赖（真实 schema + 真实 GameService/
// SessionService），只有 Wails 运行时是空的 —— 手动计时不碰窗口，这样能
// 从「点开始」一路测到「落库时长」。
func setupManualPlaySessionTest(t *testing.T, config *appconf.AppConfig) (*StartService, *sql.DB) {
	t.Helper()

	applog.SetMode(applog.ModeCLI)
	ctx := context.Background()

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("init test schema: %v", err)
	}

	now := time.Now()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO games (
			id, name, path, status, source_type, cached_at, source_id, launch_mode, created_at, updated_at
		) VALUES (?, ?, '', 'unplayed', 'bangumi', ?, '1', 'normal', ?, ?)`,
		manualSessionTestGameID, "手动计时测试", now, now, now,
	); err != nil {
		t.Fatalf("insert test game: %v", err)
	}

	sessionService := NewSessionService()
	sessionService.Init(ctx, db, config)

	gameService := NewGameService()
	gameService.Init(ctx, db, config)

	startService := NewStartService()
	startService.Init(ctx, db, config)
	startService.SetGameService(gameService)
	startService.SetSessionService(sessionService)

	return startService, db
}

// countOpenSessions 返回该游戏未结束（end_time IS NULL）的会话数。
func countOpenSessions(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM play_sessions WHERE game_id = ? AND end_time IS NULL`,
		manualSessionTestGameID,
	).Scan(&count); err != nil {
		t.Fatalf("count open sessions: %v", err)
	}
	return count
}

// totalRecordedSeconds 返回该游戏已结算的累计时长（秒）。
func totalRecordedSeconds(t *testing.T, db *sql.DB) int {
	t.Helper()
	var total sql.NullInt64
	if err := db.QueryRow(
		`SELECT SUM(duration) FROM play_sessions WHERE game_id = ? AND end_time IS NOT NULL`,
		manualSessionTestGameID,
	).Scan(&total); err != nil {
		t.Fatalf("sum recorded duration: %v", err)
	}
	if !total.Valid {
		return 0
	}
	return int(total.Int64)
}

func TestStartManualPlaySessionTracksAndFinalizes(t *testing.T) {
	startService, db := setupManualPlaySessionTest(t, &appconf.AppConfig{})

	started, err := startService.StartManualPlaySession(manualSessionTestGameID)
	if err != nil {
		t.Fatalf("StartManualPlaySession: %v", err)
	}
	if !started {
		t.Fatal("首次开始计时应当返回 started=true")
	}
	if got := countOpenSessions(t, db); got != 1 {
		t.Fatalf("应当恰好有 1 条未结束会话，实际 %d", got)
	}
	if startService.getActiveSession(manualSessionTestGameID) == nil {
		t.Fatal("会话应注册进 activeSessions，否则停止按钮会报「没有正在游玩的游戏」")
	}

	// 重复开始：不是错误，但不能再建一条 —— 否则停止时只会结算其中一条。
	startedAgain, err := startService.StartManualPlaySession(manualSessionTestGameID)
	if err != nil {
		t.Fatalf("重复开始不应报错: %v", err)
	}
	if startedAgain {
		t.Error("已在计时中时应当返回 started=false")
	}
	if got := countOpenSessions(t, db); got != 1 {
		t.Fatalf("重复开始后仍应只有 1 条会话，实际 %d", got)
	}

	// 结算时不足 60 秒的会话会被直接删除，这里把开始时间往前挪两小时。
	session := startService.getActiveSession(manualSessionTestGameID)
	session.startTime = time.Now().Add(-2 * time.Hour)

	if err := startService.EndCurrentPlaySession(manualSessionTestGameID); err != nil {
		t.Fatalf("EndCurrentPlaySession: %v", err)
	}

	if got := countOpenSessions(t, db); got != 0 {
		t.Fatalf("结束后不应残留未结束会话，实际 %d", got)
	}
	recorded := totalRecordedSeconds(t, db)
	if recorded < 7100 || recorded > 7210 {
		t.Fatalf("落库时长应约为 7200 秒，实际 %d", recorded)
	}
	if startService.getActiveSession(manualSessionTestGameID) != nil {
		t.Error("结束后应从 activeSessions 注销")
	}
}

// 回归测试：开启「仅记录活跃时长」时，手动计时没有进程可追踪活跃秒数，
// 必须回退到墙钟 —— 否则 activeSeconds 恒为 0，整条会话会被当成
// <60 秒的短会话删掉，用户点了开始却什么都没记上。
func TestManualSessionFallsBackToWallClockWhenActiveOnlyEnabled(t *testing.T) {
	config := &appconf.AppConfig{RecordActiveTimeOnly: true}
	startService, db := setupManualPlaySessionTest(t, config)

	started, err := startService.StartManualPlaySession(manualSessionTestGameID)
	if err != nil {
		t.Fatalf("StartManualPlaySession: %v", err)
	}
	if !started {
		t.Fatal("首次开始计时应当返回 started=true")
	}

	session := startService.getActiveSession(manualSessionTestGameID)
	if !session.manual {
		t.Fatal("手动计时会话必须标记 manual，否则会被按活跃时长结算")
	}
	if mode := startService.runtimeTimingMode(session); mode != GameRuntimeTimingModeWallClock {
		t.Fatalf("手动计时应上报墙钟模式，实际 %q", mode)
	}
	if seconds := startService.runtimeActiveSeconds(session, 0); seconds != nil {
		t.Fatalf("墙钟模式下不应下发 active_seconds，实际 %v", *seconds)
	}

	session.startTime = time.Now().Add(-2 * time.Hour)
	if err := startService.EndCurrentPlaySession(manualSessionTestGameID); err != nil {
		t.Fatalf("EndCurrentPlaySession: %v", err)
	}

	recorded := totalRecordedSeconds(t, db)
	if recorded < 7100 || recorded > 7210 {
		t.Fatalf("开了「仅记录活跃时长」也必须按墙钟落库约 7200 秒，实际 %d", recorded)
	}
}

// 启动游戏的会话仍然按活跃时长计时，别被手动计时的回退逻辑带偏。
func TestLaunchedSessionStillUsesActiveTime(t *testing.T) {
	config := &appconf.AppConfig{RecordActiveTimeOnly: true}
	startService, _ := setupManualPlaySessionTest(t, config)

	launched := &activePlaySession{gameID: manualSessionTestGameID}
	if mode := startService.runtimeTimingMode(launched); mode != GameRuntimeTimingModeActive {
		t.Fatalf("启动游戏的会话应保持活跃时长模式，实际 %q", mode)
	}
	if seconds := startService.runtimeActiveSeconds(launched, 42); seconds == nil || *seconds != 42 {
		t.Fatalf("活跃时长模式下应下发 active_seconds=42，实际 %v", seconds)
	}
}

func TestStartManualPlaySessionRejectsBadInput(t *testing.T) {
	startService := NewStartService()

	if _, err := startService.StartManualPlaySession("   "); err == nil {
		t.Error("空 game id 应当报错")
	}
	if _, err := startService.StartManualPlaySession("some-game"); err == nil {
		t.Error("依赖未注入时应当报错，而不是 panic 或静默成功")
	}
}
