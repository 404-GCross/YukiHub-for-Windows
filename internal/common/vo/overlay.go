package vo

// OverlayShortcut 是「呼出游戏内好友栏」快捷键的对外描述。
//
// 后端存的是 Wails 的 accelerator 形式（如 "shift+`"），界面要的是人能认的
// 写法（"Shift + ~"），所以两个都给出去，前端不用自己拼。
type OverlayShortcut struct {
	// Accelerator 是配置里的值（accelerator 形式），SetOverlayShortcut 用它回写。
	Accelerator string `json:"accelerator"`
	// Display 是 Accelerator 的展示文本，如 “Shift + ~”。
	Display string `json:"display"`
	// IsDefault 表示当前用的就是默认组合（设置页据此决定「恢复默认」是否可点）。
	IsDefault bool `json:"is_default"`
	// DefaultAccelerator / DefaultDisplay 是默认组合的两个形式。
	DefaultAccelerator string `json:"default_accelerator"`
	DefaultDisplay     string `json:"default_display"`
	// Active 是当前**真正注册成功**的组合。正常等于 Accelerator；被别的程序
	// 占用而退回备选时会是备选值；空字符串表示一个都没注册上。
	Active        string `json:"active"`
	ActiveDisplay string `json:"active_display"`
}
