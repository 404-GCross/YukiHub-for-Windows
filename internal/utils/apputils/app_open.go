package apputils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OpenDirectory 使用系统文件管理器打开指定目录
func OpenDirectory(dir string) error {
	if dir == "" {
		return os.ErrInvalid
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return exec.Command("explorer", dir).Start()
}

// OpenFile 使用系统默认应用打开指定文件。
func OpenFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return os.ErrInvalid
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return OpenDirectory(absPath)
	}

	return exec.Command("explorer", absPath).Start()
}

// OpenFileOrFolder 使用系统文件管理器打开文件或目录。如果是文件，尽量在资源管理器中选中它。
func OpenFileOrFolder(path string) error {
	if path == "" {
		return os.ErrNotExist
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return OpenDirectory(filepath.Dir(absPath))
	}

	if info.IsDir() {
		return exec.Command("explorer", absPath).Start()
	}
	return exec.Command("explorer", "/select,", absPath).Start()
}
