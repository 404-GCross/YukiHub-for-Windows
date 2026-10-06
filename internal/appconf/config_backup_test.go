package appconf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// 配置文件被写坏（中断 / 断电）时必须能从 .bak 恢复，而不是整份退回默认值。
func TestParseConfigWithBackupRecoversFromSnapshot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")

	good, err := json.Marshal(&AppConfig{MCPPort: 41234, LocaleEmulatorPath: "D:/keep-me.exe"})
	if err != nil {
		t.Fatal(err)
	}
	writeTempConfig(t, configPath+configBackupSuffix, good)
	// 主文件是半截 JSON（真实事故就是这个形态）
	broken := []byte(`{"mcp_port": 41`)
	writeTempConfig(t, configPath, broken)

	outcome, config := parseConfigWithBackup(broken, configPath, defaultAppConfig())
	if outcome != configLoadRecoveredFromBackup {
		t.Fatalf("outcome = %v, want configLoadRecoveredFromBackup", outcome)
	}
	if config.MCPPort != 41234 || config.LocaleEmulatorPath != "D:/keep-me.exe" {
		t.Fatalf("snapshot values were not restored: port=%d path=%q", config.MCPPort, config.LocaleEmulatorPath)
	}
}

// 没有 .bak（老版本升上来的第一次）且主文件坏了：必须退到默认值，但**不能**把
// 损坏原文覆盖掉，同时要留一份 .corrupt 供人工抢救。
func TestParseConfigWithBackupPreservesCorruptPrimary(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")
	broken := []byte(`{"mcp_port": 41`)
	writeTempConfig(t, configPath, broken)

	outcome, config := parseConfigWithBackup(broken, configPath, defaultAppConfig())
	if outcome != configLoadFellBackToDefaults {
		t.Fatalf("outcome = %v, want configLoadFellBackToDefaults", outcome)
	}
	if config.MCPPort != defaultAppConfig().MCPPort {
		t.Fatalf("expected defaults, got port=%d", config.MCPPort)
	}

	corrupt, err := os.ReadFile(configPath + configCorruptSuffix)
	if err != nil {
		t.Fatalf("corrupt original was not preserved: %v", err)
	}
	if string(corrupt) != string(broken) {
		t.Fatalf("corrupt copy mismatch: %q", corrupt)
	}
}

// .bak 存在但同样是坏的：同样退默认值。
func TestParseConfigWithBackupIgnoresUnusableSnapshot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")
	writeTempConfig(t, configPath+configBackupSuffix, []byte("{oops"))

	outcome, _ := parseConfigWithBackup([]byte("{oops"), configPath, defaultAppConfig())
	if outcome != configLoadFellBackToDefaults {
		t.Fatalf("outcome = %v, want configLoadFellBackToDefaults", outcome)
	}
}

// Windows 上 PowerShell / 部分编辑器会写 BOM，那不是「损坏」，不该丢设置。
func TestParseConfigWithBackupAcceptsUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")

	data, err := json.Marshal(&AppConfig{MCPPort: 31337})
	if err != nil {
		t.Fatal(err)
	}
	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, data...)

	outcome, config := parseConfigWithBackup(withBOM, configPath, defaultAppConfig())
	if outcome != configLoadPrimary {
		t.Fatalf("outcome = %v, want configLoadPrimary", outcome)
	}
	if config.MCPPort != 31337 {
		t.Fatalf("BOM input lost values: port=%d", config.MCPPort)
	}
}

// json.Unmarshal 接受 `null` 且不报错；若放过去配置会静默变成默认值。
func TestParseConfigWithBackupRejectsNullDocument(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")

	good, err := json.Marshal(&AppConfig{MCPPort: 24680})
	if err != nil {
		t.Fatal(err)
	}
	writeTempConfig(t, configPath+configBackupSuffix, good)

	outcome, config := parseConfigWithBackup([]byte("null"), configPath, defaultAppConfig())
	if outcome != configLoadRecoveredFromBackup {
		t.Fatalf("outcome = %v, want configLoadRecoveredFromBackup", outcome)
	}
	if config.MCPPort != 24680 {
		t.Fatalf("null document was accepted as config: port=%d", config.MCPPort)
	}
}

// 正常文件不应被判定为「恢复」，并且要顺带刷新快照。
func TestParseConfigWithBackupKeepsValidPrimary(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")

	data, err := json.Marshal(&AppConfig{MCPPort: 39999})
	if err != nil {
		t.Fatal(err)
	}

	outcome, config := parseConfigWithBackup(data, configPath, defaultAppConfig())
	if outcome != configLoadPrimary {
		t.Fatalf("outcome = %v, want configLoadPrimary", outcome)
	}
	if config.MCPPort != 39999 {
		t.Fatalf("unexpected port: %d", config.MCPPort)
	}

	snapshot, err := os.ReadFile(configPath + configBackupSuffix)
	if err != nil {
		t.Fatalf("expected a refreshed snapshot: %v", err)
	}
	if !isUsableConfigJSON(snapshot) {
		t.Fatal("snapshot is not a usable JSON object")
	}
}

// ensureConfigBackup 不得用坏内容覆盖已有好快照。
func TestEnsureConfigBackupKeepsValidSnapshot(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")
	backupPath := configPath + configBackupSuffix

	good, err := json.Marshal(&AppConfig{MCPPort: 12345})
	if err != nil {
		t.Fatal(err)
	}
	writeTempConfig(t, backupPath, good)
	// 主文件此刻是坏的
	writeTempConfig(t, configPath, []byte("{broken"))

	ensureConfigBackup(configPath)

	snapshot, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot) != string(good) {
		t.Fatalf("valid snapshot was overwritten: %s", snapshot)
	}
}

// 快照缺失时应从可解析的主文件补一份。
func TestEnsureConfigBackupCreatesSnapshotFromPrimary(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconf.json")

	data, err := json.Marshal(&AppConfig{MCPPort: 24680})
	if err != nil {
		t.Fatal(err)
	}
	writeTempConfig(t, configPath, data)

	ensureConfigBackup(configPath)

	snapshot, err := os.ReadFile(configPath + configBackupSuffix)
	if err != nil {
		t.Fatalf("expected snapshot to be created: %v", err)
	}
	if !isUsableConfigJSON(snapshot) {
		t.Fatalf("snapshot is not a usable JSON object: %s", snapshot)
	}
}

// writeFileAtomic 必须能覆盖已存在的文件，且不留下临时文件。
func TestWriteFileAtomicReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appconf.json")

	writeTempConfig(t, path, []byte("old"))
	if err := writeFileAtomic(path, []byte("new-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-content" {
		t.Fatalf("unexpected content: %q", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("temporary files were left behind: %v", names)
	}
}
