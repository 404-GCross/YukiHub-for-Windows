package service

import (
	"fmt"
	"strings"

	"yukihub/internal/common/vo"
)

// DefaultOverlayShortcut 是「呼出游戏内好友栏」的默认快捷键。
//
// 存的是 Wails 的 accelerator 形式：修饰键小写、`+` 分隔，主键写**物理键**
// —— ` 就是 Backquote（VK_OEM_3）。界面上展示成 “Shift + ~”（同样按 Shift 时
// 这个键打出来就是 ~），见 FormatOverlayShortcut。
//
// 为什么不是 Steam 的 Shift+Tab：那个组合被 Steam 自己占着，注册必然失败。
// 也不要用 Alt+Shift+Tab —— 它等价于「反向 Alt+Tab」，会被系统先吃掉，实测
// 连注册都被拒（日志里是 “the shortcut is already registered”）。
const DefaultOverlayShortcut = "shift+`"

// OverlayShortcutFallback 是默认组合注册失败时的备选。
//
// 全局快捷键占不到就会整个入口失效，留一个备选比让用户自己去猜要好。
const OverlayShortcutFallback = "ctrl+shift+`"

// overlayShortcutModifierLabels 是修饰键的展示名。顺序也用它来规范：
// 无论用户按什么顺序，存进去的都是 ctrl → alt → shift → cmd。
var overlayShortcutModifierLabels = []struct {
	name  string
	label string
}{
	{"ctrl", "Ctrl"},
	{"alt", "Alt"},
	{"shift", "Shift"},
	{"cmd", "Win"},
}

// overlayShortcutKeyLabels 是允许作为主键的键名 → 展示文本。
//
// 键名集合与 Wails 在 Windows 上的 winKeyCodes 对齐：不在这里的键，
// Wails 注册时会直接报「这个键不支持」，不如录的时候就说清楚。
var overlayShortcutKeyLabels = func() map[string]string {
	labels := map[string]string{
		// 符号键（写物理键名，展示时再按 Shift 换成实际符号）
		"`": "`", "-": "-", "=": "=", "[": "[", "]": "]", "\\": "\\",
		";": ";", "'": "'", ",": ",", ".": ".", "/": "/",
		// 命名键
		"space": "Space", "tab": "Tab", "return": "Enter", "enter": "Enter",
		"escape": "Esc", "backspace": "Backspace", "delete": "Delete",
		"home": "Home", "end": "End", "numlock": "NumLock",
		"left": "←", "right": "→", "up": "↑", "down": "↓",
		"page up": "PageUp", "page down": "PageDown",
	}
	for c := 'a'; c <= 'z'; c++ {
		labels[string(c)] = strings.ToUpper(string(c))
	}
	for c := '0'; c <= '9'; c++ {
		labels[string(c)] = string(c)
	}
	for i := 1; i <= 24; i++ {
		key := fmt.Sprintf("f%d", i)
		labels[key] = strings.ToUpper(key)
	}
	return labels
}()

// overlayShortcutShiftedSymbols 是「按住 Shift 时这个物理键打出来的字符」。
//
// 只影响展示：用户按的是 ` 键，看到的却是 ~ —— 说 Shift + ~ 他才好认。
var overlayShortcutShiftedSymbols = map[string]string{
	"`": "~", "1": "!", "2": "@", "3": "#", "4": "$", "5": "%",
	"6": "^", "7": "&", "8": "*", "9": "(", "0": ")",
	"-": "_", "=": "+", "[": "{", "]": "}", "\\": "|",
	";": ":", "'": "\"", ",": "<", ".": ">", "/": "?",
}

// NormalizeOverlayShortcut 把配置值/用户输入规范成 accelerator 形式。
//
// 规范化做三件事：小写、修饰键按固定顺序排列、校验键名受支持。
// 顺序固定很重要 —— 注册和注销要拿到完全一样的字符串才能对上。
func NormalizeOverlayShortcut(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", fmt.Errorf("快捷键不能为空")
	}

	components := strings.Split(trimmed, "+")
	if len(components) < 2 {
		return "", fmt.Errorf("快捷键至少要有「修饰键 + 按键」两个部分，比如 Shift + ~")
	}

	modifiers := map[string]bool{}
	key := ""
	for index, component := range components {
		component = strings.TrimSpace(component)
		if index == len(components)-1 {
			key = component
			break
		}
		switch component {
		case "ctrl", "control":
			modifiers["ctrl"] = true
		case "alt", "option":
			modifiers["alt"] = true
		case "shift":
			modifiers["shift"] = true
		case "cmd", "super", "meta", "win":
			modifiers["cmd"] = true
		default:
			return "", fmt.Errorf("不认识的修饰键：%s", component)
		}
	}

	if len(modifiers) == 0 {
		return "", fmt.Errorf("快捷键要带至少一个修饰键（Ctrl / Alt / Shift / Win）")
	}
	if _, ok := overlayShortcutKeyLabels[key]; !ok {
		return "", fmt.Errorf("不支持的按键：%s", key)
	}

	ordered := make([]string, 0, len(modifiers)+1)
	for _, modifier := range overlayShortcutModifierLabels {
		if modifiers[modifier.name] {
			ordered = append(ordered, modifier.name)
		}
	}
	return strings.Join(append(ordered, key), "+"), nil
}

// buildOverlayShortcutInfo 组装对外的快捷键描述。
func buildOverlayShortcutInfo(configured string, active string) vo.OverlayShortcut {
	normalized, err := NormalizeOverlayShortcut(configured)
	if err != nil {
		normalized = DefaultOverlayShortcut
	}
	return vo.OverlayShortcut{
		Accelerator:        normalized,
		Display:            FormatOverlayShortcut(normalized),
		IsDefault:          normalized == DefaultOverlayShortcut,
		DefaultAccelerator: DefaultOverlayShortcut,
		DefaultDisplay:     FormatOverlayShortcut(DefaultOverlayShortcut),
		Active:             active,
		ActiveDisplay:      FormatOverlayShortcut(active),
	}
}

// FormatOverlayShortcut 把 accelerator 变成界面展示文本，如 “Shift + ~”。
//
// 非法值一律退回默认组合的展示文本：与其显示一串看不懂的东西，不如显示默认值。
func FormatOverlayShortcut(accelerator string) string {
	normalized, err := NormalizeOverlayShortcut(accelerator)
	if err != nil {
		normalized = DefaultOverlayShortcut
	}

	components := strings.Split(normalized, "+")
	key := components[len(components)-1]

	labels := make([]string, 0, len(components))
	shifted := false
	for _, component := range components[:len(components)-1] {
		for _, modifier := range overlayShortcutModifierLabels {
			if modifier.name == component {
				labels = append(labels, modifier.label)
				if modifier.name == "shift" {
					shifted = true
				}
			}
		}
	}

	keyLabel := overlayShortcutKeyLabels[key]
	if shifted {
		if symbol, ok := overlayShortcutShiftedSymbols[key]; ok {
			keyLabel = symbol
		}
	}
	return strings.Join(append(labels, keyLabel), " + ")
}
