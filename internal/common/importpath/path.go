package importpath

import (
	"path/filepath"
	"runtime"
	"strings"
)

// IsWindowsAbs reports whether path looks like a Windows absolute path
// (drive letter or UNC), regardless of the host OS.
func IsWindowsAbs(path string) bool {
	path = strings.TrimSpace(path)
	if len(path) >= 3 && isDriveLetter(path[0]) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return strings.HasPrefix(path, `\\`)
}

// NormalizeWindowsSeparators converts forward slashes to backslashes and
// collapses duplicated separators. Case is preserved; it is meant for
// Windows path data that must stay stable on any host OS.
func NormalizeWindowsSeparators(path string) string {
	path = strings.ReplaceAll(strings.TrimSpace(path), "/", `\`)
	if strings.HasPrefix(path, `\\`) {
		path = `\\` + collapseBackslashes(strings.TrimLeft(path[2:], `\`))
	} else {
		path = collapseBackslashes(path)
	}
	if len(path) > 3 {
		path = strings.TrimRight(path, `\`)
	}
	return path
}

// JoinWindows joins child onto base with Windows separators. It is meant for
// path data that originates from Windows (import databases); the result keeps
// the base's case and does not depend on the host OS.
func JoinWindows(base string, child string) string {
	base = strings.TrimSpace(base)
	child = strings.TrimSpace(child)
	switch {
	case base == "":
		return child
	case child == "":
		return base
	case IsWindowsAbs(child):
		return NormalizeWindowsSeparators(child)
	}
	base = strings.TrimRight(NormalizeWindowsSeparators(base), `\`)
	child = strings.TrimLeft(NormalizeWindowsSeparators(child), `\`)
	return base + `\` + child
}

// Normalize returns a stable path key for duplicate checks.
//
// Windows 路径（盘符或 UNC）在任何宿主平台上都按 Windows 语义规范化：
// 统一反斜杠、折叠重复分隔符并小写化（Windows 文件系统大小写不敏感）。
// 其它路径按宿主语义清理；Linux 上保持大小写与原生分隔符，避免破坏大小写
// 敏感的文件系统语义。
func Normalize(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	if IsWindowsAbs(trimmed) {
		return strings.ToLower(NormalizeWindowsSeparators(trimmed))
	}

	cleaned := filepath.Clean(trimmed)
	if !filepath.IsAbs(cleaned) {
		if abs, err := filepath.Abs(cleaned); err == nil {
			cleaned = abs
		}
	}
	if runtime.GOOS == "windows" {
		cleaned = strings.ReplaceAll(cleaned, "/", `\`)
		return strings.ToLower(cleaned)
	}
	return cleaned
}

// ContainsNormalized reports whether childPath is inside parentPath, using
// the separator style of the given (already normalized) paths.
func ContainsNormalized(parentPath string, childPath string) bool {
	parentPath = strings.TrimSpace(parentPath)
	childPath = strings.TrimSpace(childPath)
	if parentPath == "" || childPath == "" || parentPath == childPath {
		return false
	}
	for _, sep := range []string{`\`, "/"} {
		parent := strings.TrimRight(parentPath, sep)
		child := strings.TrimRight(childPath, sep)
		if strings.HasPrefix(child, parent+sep) {
			return true
		}
	}
	return false
}

// Conflicts reports whether two paths refer to the same file or one contains
// the other.
func Conflicts(pathA string, pathB string) bool {
	normalizedA := Normalize(pathA)
	normalizedB := Normalize(pathB)
	if normalizedA == "" || normalizedB == "" {
		return false
	}
	if normalizedA == normalizedB {
		return true
	}
	return ContainsNormalized(normalizedA, normalizedB) || ContainsNormalized(normalizedB, normalizedA)
}

func isDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func collapseBackslashes(path string) string {
	for strings.Contains(path, `\\`) {
		path = strings.ReplaceAll(path, `\\`, `\`)
	}
	return path
}
