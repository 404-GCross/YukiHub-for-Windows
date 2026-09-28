package gamehelper

import (
	"os"
	"path/filepath"
	"strings"

	"yukihub/internal/wailsruntime"
)

// ExecutableDialogDirectory derives the initial directory for an executable
// picker. Wails v3 open dialogs do not support preselecting a filename.
func ExecutableDialogDirectory(currentPath string) string {
	currentPath = strings.TrimSpace(currentPath)
	if currentPath == "" {
		return ""
	}

	cleanPath := filepath.Clean(currentPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		absPath = cleanPath
	}

	info, err := os.Stat(absPath)
	if err == nil {
		if info.IsDir() {
			return absPath
		}
		return filepath.Dir(absPath)
	}

	if filepath.Ext(absPath) == "" {
		return ""
	}

	parentDir := filepath.Dir(absPath)
	if parentInfo, statErr := os.Stat(parentDir); statErr == nil && parentInfo.IsDir() {
		return parentDir
	}

	return ""
}

// ExecutableOpenDialogOptions builds open-dialog options for selecting a game executable.
func ExecutableOpenDialogOptions(title, defaultDirectory string) wailsruntime.OpenDialogOptions {
	return wailsruntime.OpenDialogOptions{
		Title:     title,
		Directory: defaultDirectory,
		Filters: []wailsruntime.FileFilter{
			executableFileFilter(),
			allFilesFileFilter(),
		},
	}
}

func executableFileFilter() wailsruntime.FileFilter {
	return wailsruntime.FileFilter{
		DisplayName: "Executables",
		Pattern:     "*.exe;*.bat;*.cmd;*.lnk",
	}
}

func allFilesFileFilter() wailsruntime.FileFilter {
	return wailsruntime.FileFilter{
		DisplayName: "All Files",
		Pattern:     "*.*",
	}
}
