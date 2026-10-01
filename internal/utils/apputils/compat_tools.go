package apputils

import (
	"os"
	"path/filepath"
	"strings"
)

// 兼容工具（转区 / 超分）的可执行文件名。
//
// 为什么不随安装包分发：Locale Emulator 与 Magpie 都是第三方程序，各自带着
// 自己的许可证（Magpie 是 GPL-3.0，且依赖 .NET 桌面运行时，整包上百 MB），
// 塞进我们的 AGPL 安装包既不合规也不现实。
//
// 所以这里的做法是「零配置」而不是「自带」：用户装过就自动认出来，
// 或者把整个工具目录丢到程序目录下的 compat-tools\ 里，同样能被认出来。
const (
	LocaleEmulatorExecutableName = "LEProc.exe"
	MagpieExecutableName         = "Magpie.exe"
)

// PortedCompatToolsDirName 是「把工具直接放进程序目录」时约定的子目录名。
const PortedCompatToolsDirName = "compat-tools"

// DetectLocaleEmulator 返回找到的 LEProc.exe 路径，找不到返回空串。
func DetectLocaleEmulator() string {
	return detectCompatExecutable(LocaleEmulatorExecutableName, []string{
		"Locale Emulator",
		"LocaleEmulator",
		"LE",
	})
}

// DetectMagpie 返回找到的 Magpie.exe 路径，找不到返回空串。
func DetectMagpie() string {
	return detectCompatExecutable(MagpieExecutableName, []string{
		"Magpie",
		"Magpie-*",
		"Magpie\\*",
	})
}

// CompatToolsSearchRoot 返回用户可以直接放工具的位置（程序目录下的 compat-tools）。
// 即使目录还不存在也返回，设置界面用它来提示。
func CompatToolsSearchRoot() string {
	if exe, err := GetLaunchExecutablePath(); err == nil && exe != "" {
		return filepath.Join(filepath.Dir(exe), PortedCompatToolsDirName)
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), PortedCompatToolsDirName)
	}
	return ""
}

func detectCompatExecutable(executableName string, hints []string) string {
	for _, candidate := range compatExecutableCandidates(executableName, hints) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func compatExecutableCandidates(executableName string, hints []string) []string {
	candidates := make([]string, 0, 16)
	seen := make(map[string]struct{})
	add := func(path string) {
		if path == "" {
			return
		}
		cleaned := filepath.Clean(path)
		if _, exists := seen[cleaned]; exists {
			return
		}
		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	// 1) 程序目录及其 compat-tools\tools 子树（便携版最自然的放法）
	for _, root := range executableRoots() {
		add(filepath.Join(root, executableName))
		add(filepath.Join(root, PortedCompatToolsDirName, executableName))
		add(filepath.Join(root, "tools", executableName))
		// 工具自带一层同名目录（Magpie-0.11.0\Magpie.exe）时也能命中
		for _, match := range globCompatDirs(root, hints) {
			add(filepath.Join(match, executableName))
		}
	}

	// 2) 常见安装位置
	for _, base := range []string{
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("PROGRAMFILES"),
		os.Getenv("PROGRAMFILES(X86)"),
		os.Getenv("APPDATA"),
	} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		add(filepath.Join(base, "Programs", executableName))
		for _, hint := range hints {
			if strings.Contains(hint, "*") {
				add(filepath.Join(base, hint, executableName))
				continue
			}
			add(filepath.Join(base, hint, executableName))
			add(filepath.Join(base, "Programs", hint, executableName))
		}
		for _, match := range globCompatDirs(base, hints) {
			add(filepath.Join(match, executableName))
		}
	}

	return candidates
}

func executableRoots() []string {
	roots := make([]string, 0, 2)
	if exe, err := GetLaunchExecutablePath(); err == nil && exe != "" {
		roots = append(roots, filepath.Dir(exe))
	} else if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	return roots
}

// globCompatDirs 在 base 下按 hints 找一层深度的目录（支持 * 通配），
// 用于命中 Magpie-0.11.0 这种带版本号的解压目录。
func globCompatDirs(base string, hints []string) []string {
	matches := make([]string, 0, 4)
	for _, hint := range hints {
		if !strings.Contains(hint, "*") {
			continue
		}
		found, err := filepath.Glob(filepath.Join(base, hint))
		if err != nil {
			continue
		}
		for _, match := range found {
			if info, statErr := os.Stat(match); statErr == nil && info.IsDir() {
				matches = append(matches, match)
			}
		}
	}
	return matches
}
