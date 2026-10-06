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
//
// 注意判据是「这次会话真的启动了活跃追踪器」（activeTrackStarted），
// 而不是只看配置开关 —— 只看开关会让没启动追踪器的会话恒得 0 秒。
func TestLaunchedSessionStillUsesActiveTime(t *testing.T) {
	config := &appconf.AppConfig{RecordActiveTimeOnly: true}
	startService, _ := setupManualPlaySessionTest(t, config)

	launched := &activePlaySession{gameID: manualSessionTestGameID}
	launched.activeTrackStarted.Store(true)
	if mode := startService.runtimeTimingMode(launched); mode != GameRuntimeTimingModeActive {
		t.Fatalf("启动游戏的会话应保持活跃时长模式，实际 %q", mode)
	}
	if seconds := startService.runtimeActiveSeconds(launched, 42); seconds == nil || *seconds != 42 {
		t.Fatalf("活跃时长模式下应下发 active_seconds=42，实际 %v", seconds)
	}

	// 开了开关但追踪器没起来（启动失败）→ 必须回退墙钟，否则整场会被算成 0 秒。
	notTracked := &activePlaySession{gameID: manualSessionTestGameID}
	if mode := startService.runtimeTimingMode(notTracked); mode != GameRuntimeTimingModeWallClock {
		t.Fatalf("未启动活跃追踪的会话必须回退墙钟，实际 %q", mode)
	}
	if seconds := startService.runtimeActiveSeconds(notTracked, 0); seconds != nil {
		t.Fatalf("回退墙钟时不应下发 active_seconds，实际 %v", *seconds)
	}
}

// 回归测试（会丢记录的那条）：开启「仅记录活跃时长」时，进程识别失败降级出来的
// 会话没有活跃追踪器，activeSeconds 恒为 0。旧实现只看配置开关，于是整场游玩
// 被 `<60 秒` 规则删掉 —— 用户玩了两小时，记录里什么都没有。
func TestDegradedSessionFallsBackToWallClockWhenActiveOnlyEnabled(t *testing.T) {
	config := &appconf.AppConfig{RecordActiveTimeOnly: true}
	startService, db := setupManualPlaySessionTest(t, config)

	startTime := time.Now()
	sessionID, err := startService.sessionService.CreatePendingSession(
		manualSessionTestGameID, startTime,
	)
	if err != nil {
		t.Fatalf("CreatePendingSession: %v", err)
	}
	game, err := startService.gameService.GetGameByID(manualSessionTestGameID)
	if err != nil {
		t.Fatalf("GetGameByID: %v", err)
	}
	session := startService.registerActiveSession(
		sessionID, manualSessionTestGameID, startTime, game, false,
	)

	// 进程识别失败 → 降级（不会调用 startGameFocusTracking）
	startService.degradeToForegroundTracking(session)

	if session.activeTrackStarted.Load() {
		t.Fatal("降级会话不应被标记为已启动活跃追踪")
	}
	if mode := startService.runtimeTimingMode(session); mode != GameRuntimeTimingModeWallClock {
		t.Fatalf("降级会话应上报墙钟模式（否则前端计时冻结在 00:00:00），实际 %q", mode)
	}

	session.startTime = time.Now().Add(-90 * time.Minute)
	if err := startService.EndCurrentPlaySession(manualSessionTestGameID); err != nil {
		t.Fatalf("EndCurrentPlaySession: %v", err)
	}

	recorded := totalRecordedSeconds(t, db)
	if recorded < 5300 || recorded > 5410 {
		t.Fatalf("降级会话也必须按墙钟落库约 5400 秒，实际 %d（0 表示被当短会话删了）", recorded)
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

// 进程识别失败**不能**丢掉这次游玩。
//
// 老行为是在这里直接 deleteShortOrCancelledSession（删会话），于是「启动器
// 套娃 / 进程名对不上 / 游戏秒退」全变成白玩一场 —— 这正是原分支的老问题。
// 现在改成降级：会话保留、继续计时，由「回到 YukiHub」兜底或用户手动结束。
func TestProcessDetectionFailureKeepsSessionAlive(t *testing.T) {
	startService, db := setupManualPlaySessionTest(t, &appconf.AppConfig{})

	// 模拟 startGame 已经走到「建会话 + 注册」这一步
	startTime := time.Now()
	sessionID, err := startService.sessionService.CreatePendingSession(
		manualSessionTestGameID, startTime,
	)
	if err != nil {
		t.Fatalf("CreatePendingSession: %v", err)
	}
	game, err := startService.gameService.GetGameByID(manualSessionTestGameID)
	if err != nil {
		t.Fatalf("GetGameByID: %v", err)
	}
	session := startService.registerActiveSession(
		sessionID, manualSessionTestGameID, startTime, game, false,
	)

	// 检测失败 → 降级
	startService.degradeToForegroundTracking(session)

	if !session.processUnknown.Load() {
		t.Error("会话应被标记为「无进程监控」")
	}
	if got := countOpenSessions(t, db); got != 1 {
		t.Fatalf("会话必须保留（用户其实还在玩），实际未结束会话数 = %d", got)
	}
	if sessions := startService.unknownProcessSessions(); len(sessions) != 1 {
		t.Fatalf("兜底 watcher 应能查到 1 条无进程会话，实际 %d", len(sessions))
	}

	// 依然可以手动结束，且时长正常落库
	session.startTime = time.Now().Add(-30 * time.Minute)
	if err := startService.EndCurrentPlaySession(manualSessionTestGameID); err != nil {
		t.Fatalf("EndCurrentPlaySession: %v", err)
	}
	if got := countOpenSessions(t, db); got != 0 {
		t.Fatalf("手动结束后不应残留会话，实际 %d", got)
	}
	recorded := totalRecordedSeconds(t, db)
	if recorded < 1700 || recorded > 1810 {
		t.Fatalf("应落库约 1800 秒，实际 %d", recorded)
	}
	if sessions := startService.unknownProcessSessions(); len(sessions) != 0 {
		t.Fatalf("结算后不应再有待兜底会话，实际 %d", len(sessions))
	}
}
