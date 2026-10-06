package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"yukihub/internal/appconf"
	"yukihub/internal/common/vo"
)

func voSelfSyncConfig(server, username, password string) vo.SelfSyncConfig {
	return vo.SelfSyncConfig{ServerURL: server, Username: username, Password: password}
}

// fakeSelfSyncTransport 是内存版 WebDAV，用来在无网络的情况下验证同步决策逻辑。
type fakeSelfSyncTransport struct {
	files  map[string][]byte
	writes int
}

func newFakeSelfSyncTransport() *fakeSelfSyncTransport {
	return &fakeSelfSyncTransport{files: map[string][]byte{}}
}

func (f *fakeSelfSyncTransport) TestConnection(context.Context) error { return nil }

func (f *fakeSelfSyncTransport) Exists(_ context.Context, key string) (bool, error) {
	_, ok := f.files[key]
	return ok, nil
}

func (f *fakeSelfSyncTransport) Read(_ context.Context, key string) ([]byte, error) {
	data, ok := f.files[key]
	if !ok {
		return nil, errors.New("云端文件不存在")
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (f *fakeSelfSyncTransport) Write(_ context.Context, key string, data []byte) error {
	f.writes++
	out := make([]byte, len(data))
	copy(out, data)
	f.files[key] = out
	return nil
}

func setupSelfSyncTestService(t *testing.T) (*SelfSyncService, *fakeSelfSyncTransport, *appconf.AppConfig, *sql.DB) {
	t.Helper()
	db := setupImportServiceTestDB(t)
	ctx := context.Background()
	// CurrentMetadataSource 必须给成归一化后的取值（生产路径 LoadConfig 会填默认值）：
	// appconf.SaveConfig 会把空值归一化成 vndb，若测试里留空，同步保存配置后
	// 快照里的 settings.metadata_source 会从 "" 变成 "vndb"，哈希随之变化——
	// 那是测试构造不真实，不是同步逻辑的问题。
	config := &appconf.AppConfig{
		SelfSyncURL:           "https://dav.example.com/dav",
		SelfSyncUsername:      "user",
		SelfSyncPassword:      "pass",
		CurrentMetadataSource: appconf.DefaultCurrentMetadataSource,
	}

	gameService := NewGameService()
	gameService.Init(ctx, db, config)
	sessionService := NewSessionService()
	sessionService.Init(ctx, db, config)
	importService := NewImportService()
	importService.Init(ctx, db, config)
	importService.SetGameService(gameService)
	importService.SetSessionService(sessionService)

	service := NewSelfSyncService()
	service.Init(ctx, db, config, importService)

	transport := newFakeSelfSyncTransport()
	service.transportFactory = func(_, _, _ string) (selfSyncTransport, error) {
		return transport, nil
	}
	return service, transport, config, db
}

func insertSelfSyncTestGame(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	now := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.Local)
	if _, err := db.Exec(
		`INSERT INTO games (id, name, status, source_type, source_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, "playing", "local", "", now, now); err != nil {
		t.Fatalf("插入测试游戏失败: %v", err)
	}
}

func countSelfSyncGames(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&count); err != nil {
		t.Fatalf("统计游戏数失败: %v", err)
	}
	return count
}

// 云端没有文件 → 首次上传，并且写出去的必须是 gzip 过的合法快照。
func TestSelfSyncFirstUpload(t *testing.T) {
	service, transport, config, db := setupSelfSyncTestService(t)
	insertSelfSyncTestGame(t, db, "g1", "第一次上传的游戏")

	result, err := service.SyncSelfHostedNow("")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if result.Action != "uploaded" {
		t.Fatalf("首次同步应上传，实际 %q", result.Action)
	}
	raw, ok := transport.files[selfSyncRemoteFile]
	if !ok {
		t.Fatalf("云端未写入 %s，实际文件：%v", selfSyncRemoteFile, keysOf(transport.files))
	}
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("云端文件必须是 gzip（与手机版一致）")
	}
	decoded, err := selfSyncDecompress(raw)
	if err != nil {
		t.Fatalf("解压云端文件失败: %v", err)
	}
	if err := selfSyncValidateEnvelope(decoded); err != nil {
		t.Fatalf("云端文件顶层结构不合法: %v", err)
	}
	if config.SelfSyncLastHash == "" || config.SelfSyncLastAt == "" {
		t.Fatalf("同步后必须记录基准哈希与时间")
	}
}

// 本地库为空、云端有数据 → 纯下载，**绝不能**用空库覆盖云端。
func TestSelfSyncEmptyLocalNeverOverwritesCloud(t *testing.T) {
	service, transport, config, db := setupSelfSyncTestService(t)

	// 先用另一个库造出一份"云端"快照。
	cloudDB := setupImportServiceTestDB(t)
	insertSelfSyncTestGame(t, cloudDB, "cloud-1", "云端的游戏")
	cloudSnapshot, err := buildYukiHubSnapshot(context.Background(), cloudDB, config)
	if err != nil {
		t.Fatalf("构造云端快照失败: %v", err)
	}
	transport.files[selfSyncRemoteFile] = selfSyncCompress(cloudSnapshot)
	before := len(transport.files[selfSyncRemoteFile])

	if got := countSelfSyncGames(t, db); got != 0 {
		t.Fatalf("测试前提：本地库应为空，实际 %d 条", got)
	}

	result, err := service.SyncSelfHostedNow("")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if result.Action != "downloaded" {
		t.Fatalf("空库 + 云端有数据应下载，实际 %q", result.Action)
	}
	if got := countSelfSyncGames(t, db); got != 1 {
		t.Fatalf("下载后本地应有 1 条游戏，实际 %d", got)
	}
	if transport.writes != 0 {
		t.Fatalf("空库绝不能写云端（会抹掉云端数据），实际写入 %d 次", transport.writes)
	}
	if len(transport.files[selfSyncRemoteFile]) != before {
		t.Fatalf("云端文件不应被改动")
	}
}

// 云端变了、本地没变 → 下载。
func TestSelfSyncRemoteChangeDownloads(t *testing.T) {
	service, transport, config, db := setupSelfSyncTestService(t)
	insertSelfSyncTestGame(t, db, "g1", "本地已有的游戏")

	if _, err := service.SyncSelfHostedNow(""); err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	if transport.writes != 1 {
		t.Fatalf("首次上传应写云端一次，实际 %d", transport.writes)
	}

	// 模拟云端被另一台设备改了：换一份带新游戏的快照。
	otherDB := setupImportServiceTestDB(t)
	insertSelfSyncTestGame(t, otherDB, "g1", "本地已有的游戏")
	insertSelfSyncTestGame(t, otherDB, "g2", "云端新增的游戏")
	otherSnapshot, err := buildYukiHubSnapshot(context.Background(), otherDB, config)
	if err != nil {
		t.Fatalf("构造云端快照失败: %v", err)
	}
	transport.files[selfSyncRemoteFile] = selfSyncCompress(otherSnapshot)

	result, err := service.SyncSelfHostedNow("")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if result.Action != "downloaded" {
		t.Fatalf("仅云端变化应下载，实际 %q", result.Action)
	}
	if got := countSelfSyncGames(t, db); got != 2 {
		t.Fatalf("下载后本地应有 2 条游戏，实际 %d", got)
	}
	if transport.writes != 1 {
		t.Fatalf("下载方向不应再写云端，实际写入 %d 次", transport.writes)
	}
}

// 本地变了、云端没变 → 上传。
func TestSelfSyncLocalChangeUploads(t *testing.T) {
	service, transport, _, db := setupSelfSyncTestService(t)
	insertSelfSyncTestGame(t, db, "g1", "第一版")

	if _, err := service.SyncSelfHostedNow(""); err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}

	insertSelfSyncTestGame(t, db, "g2", "本地新增")
	result, err := service.SyncSelfHostedNow("")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if result.Action != "uploaded" {
		t.Fatalf("仅本地变化应上传，实际 %q", result.Action)
	}
	if transport.writes != 2 {
		t.Fatalf("应写云端两次，实际 %d", transport.writes)
	}
}

// 两边都没变 → 已是最新，不写云端。
func TestSelfSyncNoChanges(t *testing.T) {
	service, transport, _, db := setupSelfSyncTestService(t)
	insertSelfSyncTestGame(t, db, "g1", "未变的游戏")

	if _, err := service.SyncSelfHostedNow(""); err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	writesAfterFirst := transport.writes

	result, err := service.SyncSelfHostedNow("")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if result.Action != "noop" {
		t.Fatalf("两边都没变应返回 noop，实际 %q", result.Action)
	}
	if transport.writes != writesAfterFirst {
		t.Fatalf("noop 不应写云端")
	}
}

// 两边都变过 → 先问用户（不擅自合并），带 resolution 才动手。
func TestSelfSyncConflictAsksUserThenResolves(t *testing.T) {
	t.Run("未给解决方式时只返回冲突", func(t *testing.T) {
		service, transport, config, db := setupSelfSyncTestService(t)
		insertSelfSyncTestGame(t, db, "g1", "基础游戏")
		if _, err := service.SyncSelfHostedNow(""); err != nil {
			t.Fatalf("首次上传失败: %v", err)
		}
		// 两边各自新增一条 → 双方都变了。
		insertSelfSyncTestGame(t, db, "local-new", "本地新增")
		otherDB := setupImportServiceTestDB(t)
		insertSelfSyncTestGame(t, otherDB, "g1", "基础游戏")
		insertSelfSyncTestGame(t, otherDB, "remote-new", "云端新增")
		otherSnapshot, err := buildYukiHubSnapshot(context.Background(), otherDB, config)
		if err != nil {
			t.Fatalf("构造云端快照失败: %v", err)
		}
		transport.files[selfSyncRemoteFile] = selfSyncCompress(otherSnapshot)
		writesBefore := transport.writes

		result, err := service.SyncSelfHostedNow("")
		if err != nil {
			t.Fatalf("同步失败: %v", err)
		}
		if result.Action != "conflict" {
			t.Fatalf("两边都变过应返回 conflict，实际 %q", result.Action)
		}
		if result.LocalBytes == 0 || result.RemoteBytes == 0 {
			t.Fatalf("冲突时应给出两侧字节数供界面展示，实际 local=%d remote=%d",
				result.LocalBytes, result.RemoteBytes)
		}
		if transport.writes != writesBefore {
			t.Fatalf("未决定前不得改动云端")
		}
	})

	t.Run("取消不动任何一侧", func(t *testing.T) {
		service, transport, config, db := setupSelfSyncTestService(t)
		setupConflictScenario(t, service, transport, config, db)
		writesBefore := transport.writes

		result, err := service.SyncSelfHostedNow("cancel")
		if err != nil {
			t.Fatalf("同步失败: %v", err)
		}
		if result.Action != "cancelled" {
			t.Fatalf("应返回 cancelled，实际 %q", result.Action)
		}
		if transport.writes != writesBefore {
			t.Fatalf("取消不得写云端")
		}
	})

	t.Run("用本地覆盖云端", func(t *testing.T) {
		service, transport, config, db := setupSelfSyncTestService(t)
		setupConflictScenario(t, service, transport, config, db)

		result, err := service.SyncSelfHostedNow("local")
		if err != nil {
			t.Fatalf("同步失败: %v", err)
		}
		if result.Action != "uploaded" {
			t.Fatalf("选本地应上传，实际 %q", result.Action)
		}
	})

	t.Run("用云端覆盖本地", func(t *testing.T) {
		service, transport, config, db := setupSelfSyncTestService(t)
		setupConflictScenario(t, service, transport, config, db)
		result, err := service.SyncSelfHostedNow("remote")
		if err != nil {
			t.Fatalf("同步失败: %v", err)
		}
		if result.Action != "downloaded" {
			t.Fatalf("选云端应下载，实际 %q", result.Action)
		}
		// 本机制没有删除传播（导入是纯增量），所以「用云端」只是把云端已有的导进来，
		// 本地独有的那条会保留 —— 与手机版行为一致，不是 bug。
		if got := countSelfSyncGames(t, db); got != 3 {
			t.Fatalf("用云端后本地应含双方的游戏（共 3 条），实际 %d", got)
		}
	})

	t.Run("智能合并两边都保留", func(t *testing.T) {
		service, transport, config, db := setupSelfSyncTestService(t)
		setupConflictScenario(t, service, transport, config, db)

		result, err := service.SyncSelfHostedNow("merge")
		if err != nil {
			t.Fatalf("同步失败: %v", err)
		}
		if result.Action != "merged" {
			t.Fatalf("选智能合并应返回 merged，实际 %q", result.Action)
		}
		if got := countSelfSyncGames(t, db); got != 3 {
			t.Fatalf("合并后本地应有 3 条游戏（基础 + 本地新增 + 云端新增），实际 %d", got)
		}
		raw, ok := transport.files[selfSyncRemoteFile]
		if !ok {
			t.Fatalf("合并后必须回写云端")
		}
		decoded, err := selfSyncDecompress(raw)
		if err != nil {
			t.Fatalf("解压云端失败: %v", err)
		}
		var envelope struct {
			Games []struct {
				Title string `json:"title"`
			} `json:"games"`
		}
		if err := json.Unmarshal(decoded, &envelope); err != nil {
			t.Fatalf("解析云端快照失败: %v", err)
		}
		titles := map[string]bool{}
		for _, game := range envelope.Games {
			titles[game.Title] = true
		}
		for _, want := range []string{"基础游戏", "本地新增", "云端新增"} {
			if !titles[want] {
				t.Fatalf("合并后的云端快照缺少 %q：%v", want, titles)
			}
		}
	})
}

// setupConflictScenario 造出「本地与云端各自都新增了一条」的冲突局面。
func setupConflictScenario(t *testing.T, service *SelfSyncService, transport *fakeSelfSyncTransport, config *appconf.AppConfig, db *sql.DB) {
	t.Helper()
	insertSelfSyncTestGame(t, db, "g1", "基础游戏")
	if _, err := service.SyncSelfHostedNow(""); err != nil {
		t.Fatalf("首次上传失败: %v", err)
	}
	insertSelfSyncTestGame(t, db, "local-new", "本地新增")

	otherDB := setupImportServiceTestDB(t)
	insertSelfSyncTestGame(t, otherDB, "g1", "基础游戏")
	insertSelfSyncTestGame(t, otherDB, "remote-new", "云端新增")
	otherSnapshot, err := buildYukiHubSnapshot(context.Background(), otherDB, config)
	if err != nil {
		t.Fatalf("构造云端快照失败: %v", err)
	}
	transport.files[selfSyncRemoteFile] = selfSyncCompress(otherSnapshot)
}

// 云端文件不是 YukiHub 同步文件时必须报错，而不是把垃圾导进本地库。
func TestSelfSyncRejectsForeignCloudFile(t *testing.T) {
	service, transport, _, _ := setupSelfSyncTestService(t)
	transport.files[selfSyncRemoteFile] = selfSyncCompress([]byte(`{"app":"SomethingElse"}`))

	if _, err := service.SyncSelfHostedNow(""); err == nil {
		t.Fatalf("非 YukiHub 云端文件应报错")
	}
}

// 配置不全时同步必须给出明确错误，而不是发一个必然失败的请求。
func TestSelfSyncRequiresConfiguration(t *testing.T) {
	service, _, config, _ := setupSelfSyncTestService(t)
	config.SelfSyncURL = ""

	if _, err := service.SyncSelfHostedNow(""); err == nil {
		t.Fatalf("未配置时应报错")
	}
	if err := service.TestSelfSyncConnection(voSelfSyncConfig("", "u", "p")); err == nil {
		t.Fatalf("配置不完整时测试连接应报错")
	}
}

// 地址缺协议头时补 https（与手机版 validateServerUrl 一致）。
func TestSelfSyncServerURLNormalization(t *testing.T) {
	service, _, config, _ := setupSelfSyncTestService(t)
	if err := service.SaveSelfSyncConfig(voSelfSyncConfig("dav.jianguoyun.com/dav", "u", "p")); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if !strings.HasPrefix(config.SelfSyncURL, "https://") {
		t.Fatalf("缺协议头应补 https://，实际 %q", config.SelfSyncURL)
	}

	if err := service.SaveSelfSyncConfig(voSelfSyncConfig("http://192.168.1.2/dav", "u", "p")); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if config.SelfSyncURL != "http://192.168.1.2/dav" {
		t.Fatalf("已带协议头不得改写，实际 %q", config.SelfSyncURL)
	}
}

func keysOf(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	return keys
}
