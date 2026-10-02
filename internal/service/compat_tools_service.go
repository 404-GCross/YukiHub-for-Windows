package service

import (
	"context"
	"os"
	"strings"

	"yukihub/internal/appconf"
	"yukihub/internal/applog"
	"yukihub/internal/utils/apputils"
)

// CompatToolsDetection 是自动检测到的转区 / 超分工具路径。
//
// YukiHub 自带一份 Locale Emulator 与 Magpie（放在程序目录的 compat-tools\ 下，
// 见该目录的 SOURCE.txt），所以正常情况下用户什么都不用配；
// 检测同时覆盖「系统里已经装过」「用户自己换了新版本」两种情况。
type CompatToolsDetection struct {
	LocaleEmulatorPath string `json:"localeEmulatorPath"`
	MagpiePath         string `json:"magpiePath"`
	// PortableToolsDir 是「把工具丢进这里就能被认出来」的目录，供设置界面提示。
	PortableToolsDir string `json:"portableToolsDir"`
}

// DetectCompatTools 扫描常见位置，返回找到的 Locale Emulator / Magpie 路径。
func (s *GameService) DetectCompatTools() CompatToolsDetection {
	return CompatToolsDetection{
		LocaleEmulatorPath: apputils.DetectLocaleEmulator(),
		MagpiePath:         apputils.DetectMagpie(),
		PortableToolsDir:   apputils.CompatToolsSearchRoot(),
	}
}

// ApplyDetectedCompatTools 在配置里没有可用路径时，用自动检测结果补齐并落盘。
//
// 「没有可用路径」包括两种情况：
//   - 路径为空（首次运行，或用户清过配置）
//   - 路径已经失效（换过安装目录、或手动删掉了旧工具）
//
// 只在这两种情况下写入：用户手动指了一个**真实存在**的路径就不再覆盖，
// 免得把他的选择改掉。
func ApplyDetectedCompatTools(config *appconf.AppConfig) bool {
	if config == nil {
		return false
	}
	changed := false
	if compatToolPathUnusable(config.LocaleEmulatorPath) {
		if detected := apputils.DetectLocaleEmulator(); detected != "" && detected != config.LocaleEmulatorPath {
			config.LocaleEmulatorPath = detected
			changed = true
		}
	}
	if compatToolPathUnusable(config.MagpiePath) {
		if detected := apputils.DetectMagpie(); detected != "" && detected != config.MagpiePath {
			config.MagpiePath = detected
			changed = true
		}
	}
	if !changed {
		return false
	}
	if err := appconf.SaveConfig(config); err != nil {
		applog.LogErrorf(context.Background(), "failed to persist auto-detected compat tools: %v", err)
		return false
	}
	applog.LogInfof(context.Background(), "auto-detected compat tools: localeEmulator=%q magpie=%q",
		config.LocaleEmulatorPath, config.MagpiePath)
	return true
}

// compatToolPathUnusable 判断路径是否为空、或者指向的文件已经不在了。
func compatToolPathUnusable(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return true
	}
	info, err := os.Stat(trimmed)
	return err != nil || info.IsDir()
}
