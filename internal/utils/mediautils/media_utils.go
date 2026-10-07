package mediautils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yukihub/internal/utils/apputils"
)

// trailerExtensions 是大屏模式允许的本地预告片容器格式。
// WebView2 对 mp4 / webm 支持最好，其余格式能否解码取决于系统编解码器。
var trailerExtensions = []string{".mp4", ".webm", ".mkv", ".mov", ".avi", ".m4v"}

// trailerDirOverride 仅用于测试，覆盖数据目录下的默认 trailers 目录。
var trailerDirOverride string

// TrailerExtensions 返回允许的扩展名副本，供文件对话框过滤器复用。
func TrailerExtensions() []string {
	return append([]string(nil), trailerExtensions...)
}

// SetTrailersDirForTest 临时覆盖受管预告片目录，供测试隔离真实数据目录；返回恢复函数。
func SetTrailersDirForTest(dir string) func() {
	previous := trailerDirOverride
	trailerDirOverride = dir
	return func() {
		trailerDirOverride = previous
	}
}

// TrailersDir 返回受管的预告片目录（数据目录下的 trailers/），并确保其存在。
func TrailersDir() (string, error) {
	if trailerDirOverride != "" {
		if err := os.MkdirAll(trailerDirOverride, os.ModePerm); err != nil {
			return "", err
		}
		return trailerDirOverride, nil
	}

	appDir, err := apputils.GetDataDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(appDir, "trailers")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	return dir, nil
}

// IsSupportedTrailerExt 判断扩展名是否在允许列表内（大小写不敏感）。
func IsSupportedTrailerExt(ext string) bool {
	normalized := strings.ToLower(strings.TrimSpace(ext))
	for _, candidate := range trailerExtensions {
		if normalized == candidate {
			return true
		}
	}
	return false
}

// SaveTrailer 把用户选中的视频复制进受管目录，返回可直接给前端的
// /local/trailers/<gameID><ext> 地址（与封面 /local/covers/... 同款约定）。
func SaveTrailer(srcPath, gameID string) (string, error) {
	if strings.TrimSpace(gameID) == "" {
		return "", fmt.Errorf("game id is required")
	}

	ext := strings.ToLower(filepath.Ext(srcPath))
	if !IsSupportedTrailerExt(ext) {
		return "", fmt.Errorf("unsupported trailer format: %s", ext)
	}

	dir, err := TrailersDir()
	if err != nil {
		return "", err
	}

	// 换格式重选时清掉旧文件，避免同 gameID 残留多个视频
	removeTrailersWithBaseName(dir, gameID)

	destFileName := gameID + ext
	if err := apputils.CopyFile(srcPath, filepath.Join(dir, destFileName)); err != nil {
		return "", err
	}
	return "/local/trailers/" + destFileName, nil
}

// RemoveTrailer 删除某个游戏已保存的预告片文件（不存在时静默成功）。
func RemoveTrailer(gameID string) error {
	if strings.TrimSpace(gameID) == "" {
		return nil
	}

	dir, err := TrailersDir()
	if err != nil {
		return err
	}
	removeTrailersWithBaseName(dir, gameID)
	return nil
}

func removeTrailersWithBaseName(dir, baseName string) {
	for _, ext := range trailerExtensions {
		_ = os.Remove(filepath.Join(dir, baseName+ext))
	}
}

// introVideoBaseName 是入场视频在受管目录里的固定文件名。
// 大屏同时只用一个入场视频（对齐手机端 bigscreen_intro_video 单值），
// 换文件时先清旧的，避免数据目录里堆一堆没人用的视频。
const introVideoBaseName = "intro"

// IntroVideoDir 返回受管的入场视频目录（数据目录下的 intro/），并确保其存在。
func IntroVideoDir() (string, error) {
	if trailerDirOverride != "" {
		// 测试里覆盖了 trailers 目录时就近复用它的兄弟目录
		dir := trailerDirOverride + "-intro"
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return "", err
		}
		return dir, nil
	}

	appDir, err := apputils.GetDataDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(appDir, "intro")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	return dir, nil
}

// SaveIntroVideo 把用户选中的视频复制进受管目录，返回 /local/intro/<name><ext>。
func SaveIntroVideo(srcPath string) (string, error) {
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return "", fmt.Errorf("source path is required")
	}

	ext := strings.ToLower(filepath.Ext(srcPath))
	if !IsSupportedTrailerExt(ext) {
		return "", fmt.Errorf("unsupported intro video format: %s", ext)
	}

	dir, err := IntroVideoDir()
	if err != nil {
		return "", err
	}

	removeTrailersWithBaseName(dir, introVideoBaseName)

	destFileName := introVideoBaseName + ext
	if err := apputils.CopyFile(srcPath, filepath.Join(dir, destFileName)); err != nil {
		return "", err
	}
	return "/local/intro/" + destFileName, nil
}

// RemoveIntroVideo 删掉已保存的入场视频（不存在时静默成功）。
func RemoveIntroVideo() error {
	dir, err := IntroVideoDir()
	if err != nil {
		return err
	}
	removeTrailersWithBaseName(dir, introVideoBaseName)
	return nil
}
