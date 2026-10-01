package service

import (
	"context"

	"yukihub/internal/appconf"
	"yukihub/internal/applog"
	"yukihub/internal/utils/apputils"
)

// CompatToolsDetection 是自动检测到的转区 / 超分工具路径。
//
// 为什么需要它：这两样都是第三方程序，不能随我们的安装包分发（各自的许可证 +
// Magpie 还依赖 .NET 桌面运行时）。所以做成「零配置」——用户装过就自动认出来，
// 或者把整个工具目录丢到程序目录下的 compat-tools\ 里也能被认出来。
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

// ApplyDetectedCompatTools 在配置里缺少路径时，用自动检测结果补齐并落盘。
//
// 只在「原本为空」时写入：用户手动指过路径（哪怕指错了）就不覆盖，
// 免得把他的选择改掉。
func ApplyDetectedCompatTools(config *appconf.AppConfig) bool {
	if config == nil {
		return false
	}
	changed := false
	if config.LocaleEmulatorPath == "" {
		if detected := apputils.DetectLocaleEmulator(); detected != "" {
			config.LocaleEmulatorPath = detected
			changed = true
		}
	}
	if config.MagpiePath == "" {
		if detected := apputils.DetectMagpie(); detected != "" {
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
