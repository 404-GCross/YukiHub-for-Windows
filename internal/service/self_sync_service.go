package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"yukihub/internal/appconf"
	"yukihub/internal/applog"
	"yukihub/internal/common/vo"
	"yukihub/internal/service/cloudprovider/webdav"
	"yukihub/internal/wailsruntime"
)

// 自持同步（WebDAV）常量。
//
// 目录与文件名必须与手机版逐字一致：手机版 SyncManager 写的是
// `YukiHub/YukiHub_sync.json`（REMOTE_DIR + REMOTE_FILE）。桌面端若换个名字，
// 两端就会「各同步各的」—— 而且不会报任何错，用户只会觉得同步没生效。
const (
	selfSyncRemoteDir  = "YukiHub"
	selfSyncRemoteFile = "YukiHub/YukiHub_sync.json"

	// selfSyncAppliedEvent：本地库被同步结果改动（下载 / 合并）后广播，
	// 前端据此失效游戏库缓存并刷新首页。
	selfSyncAppliedEvent = "yukihub-sync:applied"
	// selfSyncStatusEvent：配置或上次同步状态变化，供设置面板回显。
	selfSyncStatusEvent = "self-sync:status-changed"

	// 冲突解决方式，取值与手机版 SyncManager 的 RESOLVE_* 一一对应。
	selfSyncResolveMerge  = "merge"
	selfSyncResolveLocal  = "local"
	selfSyncResolveRemote = "remote"
	selfSyncResolveCancel = "cancel"
)

// selfSyncAutoInterval：自动同步的最小间隔，与手机版 maybeAutoWebDavSync 一致（10 分钟）。
const selfSyncAutoInterval = 10 * time.Minute

// selfSyncTransport 抽象自持同步用到的 WebDAV 能力，便于单测注入假实现。
type selfSyncTransport interface {
	TestConnection(ctx context.Context) error
	Exists(ctx context.Context, key string) (bool, error)
	Read(ctx context.Context, key string) ([]byte, error)
	Write(ctx context.Context, key string, data []byte) error
}

// SelfSyncService 把与手机版**完全相同**的 schema 5 快照同步到用户自建的 WebDAV，
// 对应手机版 SyncManager 的 `sync()`。
//
// 与「账号云同步」（yukihub.zh.kg）的区别只在传输通道：快照的构造、导入语义、
// 冲突判定全部共用同一份实现（buildYukiHubSnapshot / importYukiHubSnapshot），
// 所以两边同步出去的数据必须一致 —— 这是「传给云端的数据要和手机版一致」的前提。
type SelfSyncService struct {
	ctx     context.Context
	db      *sql.DB
	config  *appconf.AppConfig
	runtime wailsruntime.Runtime
	imports *ImportService

	mu     sync.Mutex
	syncMu sync.Mutex

	lastSyncAt time.Time
	now        func() time.Time

	// transportFactory 只在测试里替换；生产路径为 nil，走真实 WebDAV。
	transportFactory func(serverURL, username, password string) (selfSyncTransport, error)
}

func NewSelfSyncService() *SelfSyncService {
	return &SelfSyncService{
		runtime: wailsruntime.Unavailable(),
		now:     time.Now,
	}
}

// Init 注入运行时依赖。imports 用于把云端快照落回本地库（复用现成的导入器）。

//wails:ignore
func (s *SelfSyncService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig, imports *ImportService) {
	s.ctx = ctx
	s.db = db
	s.config = config
	s.imports = imports
	if s.now == nil {
		s.now = time.Now
	}
}

//wails:ignore
func (s *SelfSyncService) SetRuntime(runtime wailsruntime.Runtime) {
	if runtime != nil {
		s.runtime = runtime
	}
}

// ==================== 配置 ====================

// GetSelfSyncConfig 回显 WebDAV 配置（含密码，手机版同样把密码填回输入框）。
func (s *SelfSyncService) GetSelfSyncConfig() vo.SelfSyncConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config == nil {
		return vo.SelfSyncConfig{}
	}
	return vo.SelfSyncConfig{
		ServerURL:    strings.TrimSpace(s.config.SelfSyncURL),
		Username:     strings.TrimSpace(s.config.SelfSyncUsername),
		Password:     s.config.SelfSyncPassword,
		AutoSync:     s.config.SelfSyncAutoSync,
		Configured:   selfSyncConfigured(s.config),
		LastSyncAt:   s.config.SelfSyncLastAt,
		LastSyncHash: s.config.SelfSyncLastHash,
	}
}

// SaveSelfSyncConfig 保存 WebDAV 配置。
//
// 校验规则与手机版 WebDavSettingsDialog.saveConfig 一致：三项都必填，
// 地址缺协议头时自动补 `https://`（坚果云等只接受 https）。
func (s *SelfSyncService) SaveSelfSyncConfig(cfg vo.SelfSyncConfig) error {
	server := strings.TrimSpace(cfg.ServerURL)
	username := strings.TrimSpace(cfg.Username)
	password := cfg.Password

	if server == "" {
		return fmt.Errorf("请输入 WebDAV 服务器地址")
	}
	if username == "" {
		return fmt.Errorf("请输入用户名")
	}
	if password == "" {
		return fmt.Errorf("请输入密码 / 应用密码")
	}
	server = normalizeSelfSyncServerURL(server)

	s.mu.Lock()
	if s.config == nil {
		s.mu.Unlock()
		return errors.New("配置尚未就绪")
	}
	s.config.SelfSyncURL = server
	s.config.SelfSyncUsername = username
	s.config.SelfSyncPassword = password
	s.config.SelfSyncAutoSync = cfg.AutoSync
	config := s.config
	s.mu.Unlock()

	if err := appconf.SaveConfig(config); err != nil {
		return fmt.Errorf("保存 WebDAV 配置失败: %w", err)
	}
	s.emitStatus()
	return nil
}

// TestSelfSyncConnection 用**表单里的**配置测一次连通性，不影响已保存的配置。
func (s *SelfSyncService) TestSelfSyncConnection(cfg vo.SelfSyncConfig) error {
	server := normalizeSelfSyncServerURL(strings.TrimSpace(cfg.ServerURL))
	username := strings.TrimSpace(cfg.Username)
	password := cfg.Password
	if server == "" || username == "" || password == "" {
		return fmt.Errorf("请填写完整的 WebDAV 配置")
	}
	transport, err := s.transportFor(server, username, password)
	if err != nil {
		return err
	}
	if err := transport.TestConnection(s.resolveContext(nil)); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	return nil
}

// ==================== 同步 ====================

// SyncSelfHostedNow 立刻与 WebDAV 同步一次。
//
// resolution 为空时：若判定为冲突，**不擅自合并**，而是返回 Action=conflict 与
// 两侧字节数，交由界面询问用户（对齐手机版 WebDavSettingsDialog 的冲突对话框）。
// 用户选完后再带 resolution 调一次：merge / local / remote / cancel。
func (s *SelfSyncService) SyncSelfHostedNow(resolution string) (vo.SelfSyncResult, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	return s.syncLocked(resolution)
}

// RunStartupSelfSync 启动时的自动同步：已配置 + 开了自动同步 + 距上次超过 10 分钟
// 才跑；冲突按手机版 maybeAutoWebDavSync 的做法默认「智能合并」。
func (s *SelfSyncService) RunStartupSelfSync() {
	s.mu.Lock()
	configured := selfSyncConfigured(s.config)
	autoSync := s.config != nil && s.config.SelfSyncAutoSync
	lastAt := ""
	if s.config != nil {
		lastAt = s.config.SelfSyncLastAt
	}
	s.mu.Unlock()

	if !configured || !autoSync {
		return
	}
	if parsed, err := time.Parse(time.RFC3339, lastAt); err == nil {
		if s.now().Sub(parsed) < selfSyncAutoInterval {
			return
		}
	}

	go func() {
		s.syncMu.Lock()
		defer s.syncMu.Unlock()
		result, err := s.syncLocked(selfSyncResolveMerge)
		if err != nil {
			applog.LogWarningf(s.ctx, "自持同步：自动同步失败（忽略）：%v", err)
			return
		}
		applog.LogInfof(s.ctx, "自持同步：自动同步完成 action=%s games=%d sessions=%d",
			result.Action, result.Games, result.Sessions)
	}()
}

func (s *SelfSyncService) syncLocked(resolution string) (vo.SelfSyncResult, error) {
	s.mu.Lock()
	if s.config == nil || !selfSyncConfigured(s.config) {
		s.mu.Unlock()
		return vo.SelfSyncResult{}, errors.New("请先配置 WebDAV 服务器地址、用户名与密码")
	}
	server := s.config.SelfSyncURL
	username := s.config.SelfSyncUsername
	password := s.config.SelfSyncPassword
	lastHash := strings.TrimSpace(s.config.SelfSyncLastHash)
	config := s.config
	s.mu.Unlock()

	transport, err := s.transportFor(server, username, password)
	if err != nil {
		return vo.SelfSyncResult{}, err
	}
	ctx := s.resolveContext(nil)

	localSnapshot, err := buildYukiHubSnapshot(ctx, s.db, config)
	if err != nil {
		return vo.SelfSyncResult{}, err
	}
	localHash := hashSnapshot(localSnapshot)

	result := vo.SelfSyncResult{LocalBytes: len(localSnapshot)}

	remoteExists, err := transport.Exists(ctx, selfSyncRemoteFile)
	if err != nil {
		return result, fmt.Errorf("检查云端文件失败: %w", err)
	}

	var remoteRaw []byte
	remoteHash := ""
	if remoteExists {
		compressed, readErr := transport.Read(ctx, selfSyncRemoteFile)
		if readErr != nil {
			return result, fmt.Errorf("读取云端快照失败: %w", readErr)
		}
		remoteRaw, err = selfSyncDecompress(compressed)
		if err != nil {
			return result, fmt.Errorf("解析云端快照失败: %w", err)
		}
		if err := selfSyncValidateEnvelope(remoteRaw); err != nil {
			return result, err
		}
		remoteHash = hashSnapshot(remoteRaw)
		result.RemoteBytes = len(remoteRaw)
	}

	localChanged := lastHash == "" || localHash != lastHash
	remoteChanged := !remoteExists || lastHash == "" || remoteHash != lastHash

	// 先把「不需要用户拍板」的分支走完，顺序与手机版 SyncManager.sync 一致。
	switch {
	case !remoteExists:
		return s.finishSelfSync(transport, result, "uploaded", localSnapshot, localHash, config, false)

	case snapshotIsEmpty(localSnapshot) && !snapshotIsEmpty(remoteRaw):
		// 本地库为空、云端有数据 → 纯下载。
		//
		// 这条对应手机版「新设备首次同步」分支，并且刻意放宽了「必须是从没同步过」的
		// 条件：本机制没有任何删除传播（导入纯增量、无墓碑），而空库恰恰是重装 /
		// 换设备 / 手滑清库后最需要它的时候。不加这条，清空游戏库后点同步会走上传，
		// 用空库把云端抹掉。
		imported, importErr := importYukiHubSnapshot(ctx, s.imports, remoteRaw)
		if importErr != nil {
			return result, importErr
		}
		result.Imported = imported.Success
		result.Skipped = imported.Skipped
		result.Failed = imported.Failed
		return s.finishSelfSync(transport, result, "downloaded", remoteRaw, remoteHash, config, true)

	case !localChanged && !remoteChanged:
		return s.finishSelfSync(transport, result, "noop", localSnapshot, localHash, config, false)

	case localChanged && !remoteChanged:
		return s.finishSelfSync(transport, result, "uploaded", localSnapshot, localHash, config, false)

	case !localChanged && remoteChanged:
		imported, importErr := importYukiHubSnapshot(ctx, s.imports, remoteRaw)
		if importErr != nil {
			return result, importErr
		}
		result.Imported = imported.Success
		result.Skipped = imported.Skipped
		result.Failed = imported.Failed
		return s.finishSelfSync(transport, result, "downloaded", remoteRaw, remoteHash, config, true)
	}

	// 两边都变过 → 冲突。没给解决方式时先问用户。
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "":
		result.Action = "conflict"
		return result, nil
	case selfSyncResolveCancel:
		result.Action = "cancelled"
		return result, nil
	case selfSyncResolveRemote:
		imported, importErr := importYukiHubSnapshot(ctx, s.imports, remoteRaw)
		if importErr != nil {
			return result, importErr
		}
		result.Imported = imported.Success
		result.Skipped = imported.Skipped
		result.Failed = imported.Failed
		return s.finishSelfSync(transport, result, "downloaded", remoteRaw, remoteHash, config, true)
	case selfSyncResolveLocal:
		return s.finishSelfSync(transport, result, "uploaded", localSnapshot, localHash, config, false)
	default:
		// 智能合并：把云端导进来（云端资料覆盖本地自动生成的文件夹名），
		// 再把合并后的本地库整体上传 —— 与手机版 mergeSnapshots 同一策略。
		imported, importErr := importYukiHubSnapshot(ctx, s.imports, remoteRaw)
		if importErr != nil {
			return result, importErr
		}
		result.Imported = imported.Success
		result.Skipped = imported.Skipped
		result.Failed = imported.Failed
		merged, buildErr := buildYukiHubSnapshot(ctx, s.db, config)
		if buildErr != nil {
			return result, buildErr
		}
		return s.finishSelfSync(transport, result, "merged", merged, hashSnapshot(merged), config, true)
	}
}

// finishSelfSync 在需要写云端时上传、更新基准哈希与时间，并在本地库被改动时广播刷新。
func (s *SelfSyncService) finishSelfSync(
	transport selfSyncTransport,
	result vo.SelfSyncResult,
	action string,
	syncedSnapshot []byte,
	syncedHash string,
	config *appconf.AppConfig,
	localTouched bool,
) (vo.SelfSyncResult, error) {
	if action == "uploaded" || action == "merged" {
		if err := transport.Write(s.resolveContext(nil), selfSyncRemoteFile, selfSyncCompress(syncedSnapshot)); err != nil {
			return result, fmt.Errorf("写入云端快照失败: %w", err)
		}
	}

	games, sessions := countSnapshotEntries(syncedSnapshot)
	result.Action = action
	result.Games = games
	result.Sessions = sessions
	result.SyncedAt = s.now().Format(time.RFC3339)

	s.mu.Lock()
	s.lastSyncAt = s.now()
	if config != nil {
		config.SelfSyncLastHash = syncedHash
		config.SelfSyncLastAt = result.SyncedAt
	}
	s.mu.Unlock()
	if config != nil {
		if err := appconf.SaveConfig(config); err != nil {
			applog.LogErrorf(s.ctx, "自持同步：保存配置失败: %v", err)
		}
	}
	s.emitStatus()

	if localTouched {
		s.emitApplied(result)
	}
	return result, nil
}

// ==================== 内部工具 ====================

func selfSyncConfigured(config *appconf.AppConfig) bool {
	if config == nil {
		return false
	}
	return strings.TrimSpace(config.SelfSyncURL) != "" &&
		strings.TrimSpace(config.SelfSyncUsername) != "" &&
		config.SelfSyncPassword != ""
}

// normalizeSelfSyncServerURL 与手机版 validateServerUrl 一致：缺协议头时补 https。
func normalizeSelfSyncServerURL(server string) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return ""
	}
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		return "https://" + server
	}
	return server
}

// selfSyncValidateEnvelope 只校验顶层 app 字段，与手机版同一松紧度
// （手机版：`if (!"YukiHub".equals(remote.optString("app", ""))) throw`）。
func selfSyncValidateEnvelope(raw []byte) error {
	var envelope struct {
		App string `json:"app"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("云端文件不是有效的同步文件: %w", err)
	}
	if envelope.App != "YukiHub" {
		return errors.New("云端文件不是有效的 YukiHub 同步文件")
	}
	return nil
}

func selfSyncCompress(raw []byte) []byte {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return raw
	}
	if err := writer.Close(); err != nil {
		return raw
	}
	return buf.Bytes()
}

// selfSyncDecompress 解压 gzip；不是 gzip 就原样返回（对齐手机版 decompressIfGzip）。
func selfSyncDecompress(raw []byte) ([]byte, error) {
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		return raw, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (s *SelfSyncService) resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *SelfSyncService) transportFor(serverURL, username, password string) (selfSyncTransport, error) {
	if s.transportFactory != nil {
		return s.transportFactory(serverURL, username, password)
	}
	cfg := webdav.Config{URL: serverURL, Username: username, Password: password}
	if s.config != nil {
		cfg.ProxyConfig = s.config
	}
	provider, err := webdav.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("WebDAV 配置无效: %w", err)
	}
	return &webdavSelfSyncTransport{provider: provider}, nil
}

func (s *SelfSyncService) emitStatus() {
	if s.runtime == nil {
		return
	}
	s.runtime.Emit(selfSyncStatusEvent, s.GetSelfSyncConfig())
}

func (s *SelfSyncService) emitApplied(result vo.SelfSyncResult) {
	if s.runtime == nil {
		return
	}
	s.runtime.Emit(selfSyncAppliedEvent, result)
}

// ==================== WebDAV 传输实现 ====================

// webdavSelfSyncTransport 用云存储 Provider 实现快照的读写。
//
// Provider 的接口是按「本地文件路径」设计的，所以每次读写都落一个临时文件；
// 快照本身只有几百 KB，这点开销可以忽略。
type webdavSelfSyncTransport struct {
	provider *webdav.Provider
}

func (t *webdavSelfSyncTransport) TestConnection(ctx context.Context) error {
	return t.provider.TestConnection(ctx)
}

func (t *webdavSelfSyncTransport) Exists(ctx context.Context, key string) (bool, error) {
	keys, err := t.provider.ListObjects(ctx, selfSyncRemoteDir)
	if err != nil {
		return false, err
	}
	want := strings.Trim(strings.TrimSpace(key), "/")
	for _, item := range keys {
		if strings.Trim(strings.TrimSpace(item), "/") == want {
			return true, nil
		}
	}
	return false, nil
}

func (t *webdavSelfSyncTransport) Read(ctx context.Context, key string) ([]byte, error) {
	tempFile, err := os.CreateTemp("", "yukihub-self-sync-*.gz")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer func() { _ = os.Remove(tempPath) }()

	if err := t.provider.DownloadFile(ctx, key, tempPath); err != nil {
		return nil, err
	}
	return os.ReadFile(tempPath)
}

func (t *webdavSelfSyncTransport) Write(ctx context.Context, key string, data []byte) error {
	tempFile, err := os.CreateTemp("", "yukihub-self-sync-*.gz")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	// 用户可能还没有建 YukiHub 目录；UploadFile 遇到 404/409 会自动补建父目录。
	return t.provider.UploadFile(ctx, key, tempPath)
}
