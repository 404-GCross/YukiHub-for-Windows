package mediautils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yukihub/internal/utils/apputils"
)

// trailerExtensions 是大屏模式允许的本地预告片容器格式。
// WebView2 对 mp4 / webm 支持最好，其余格式能否解码取决于系统编解码器。
var trailerExtensions = []string{".mp4", ".webm", ".mkv", ".mov", ".avi", ".m4v"}

// imageExtensions 是大屏自定义图允许的格式，对齐手机端 BigScreenArt.guessExt
// （png / jpg / jpeg / webp；gif 在手机端同样不支持）。
var imageExtensions = []string{".png", ".jpg", ".jpeg", ".webp"}

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

// 大屏自定义图（对齐手机版 M10 BigScreenArt）：标题图（Steam 式 logo）与背景图。
// 文件都复制进受管目录 <数据目录>/bigscreen/art/，路径（/local/... 地址）写进 games 表的
// logo_path / bg_path。两列与 trailer_path 一样只在本地使用：手机版 exportGamesJson
// 的导出清单里本来就没有这两列，本地图不入快照，防止路径污染别的设备。
const (
	GameArtKindLogo = "logo"
	GameArtKindBg   = "bg"
)

// gameArtURLPrefix 是自定义图对外的地址前缀（LocalFileHandler 服务数据目录下任意文件）。
const gameArtURLPrefix = "/local/bigscreen/art/"

// artDirOverride 仅用于测试，覆盖数据目录下的默认 bigscreen/art 目录。
var artDirOverride string

// SetGameArtDirForTest 临时覆盖受管自定义图目录，供测试隔离真实数据目录；返回恢复函数。
func SetGameArtDirForTest(dir string) func() {
	previous := artDirOverride
	artDirOverride = dir
	return func() {
		artDirOverride = previous
	}
}

// GameArtDir 返回受管的自定义图目录（数据目录下的 bigscreen/art/），并确保其存在。
func GameArtDir() (string, error) {
	if artDirOverride != "" {
		if err := os.MkdirAll(artDirOverride, os.ModePerm); err != nil {
			return "", err
		}
		return artDirOverride, nil
	}

	appDir, err := apputils.GetDataDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(appDir, "bigscreen", "art")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	return dir, nil
}

// IsSupportedImageExt 判断扩展名是否是允许的自定义图格式（大小写不敏感）。
func IsSupportedImageExt(ext string) bool {
	normalized := strings.ToLower(strings.TrimSpace(ext))
	for _, candidate := range imageExtensions {
		if normalized == candidate {
			return true
		}
	}
	return false
}

// IsGameArtKind 判断 kind 是否是合法的自定义图类型。
func IsGameArtKind(kind string) bool {
	return kind == GameArtKindLogo || kind == GameArtKindBg
}

// SaveGameArt 把用户选中的图片复制进受管目录，返回可直接给前端的
// /local/bigscreen/art/<kind>_<gameID>_<毫秒时间戳><ext> 地址。
//
// 文件名带毫秒时间戳（对齐手机端 kind_gameId_timestamp.ext）：换图后地址必然变化，
// LocalFileHandler 的 max-age=1y 强缓存不会把旧图端出来，前端不用自己拼 cache-bust 参数。
func SaveGameArt(srcPath, gameID, kind string) (string, error) {
	if strings.TrimSpace(gameID) == "" {
		return "", fmt.Errorf("game id is required")
	}
	if !IsGameArtKind(kind) {
		return "", fmt.Errorf("invalid art kind: %s", kind)
	}

	ext := strings.ToLower(filepath.Ext(srcPath))
	if !IsSupportedImageExt(ext) {
		return "", fmt.Errorf("unsupported image format: %s", ext)
	}

	dir, err := GameArtDir()
	if err != nil {
		return "", err
	}

	destFileName := fmt.Sprintf("%s_%s_%d%s", kind, gameID, time.Now().UnixMilli(), ext)
	if err := apputils.CopyFile(srcPath, filepath.Join(dir, destFileName)); err != nil {
		return "", err
	}
	return gameArtURLPrefix + destFileName, nil
}

// RemoveGameArt 删除受管目录里的自定义图文件（不存在时静默成功）。
// 只接受 gameArtURLPrefix 下的地址，且用 Base 挡掉路径穿越；其余输入一律忽略，
// 对齐手机端 BigScreenArt.delete「只删自己目录里的文件」的防误删约定。
func RemoveGameArt(artURL string) error {
	artURL = strings.TrimSpace(artURL)
	if artURL == "" || !strings.HasPrefix(artURL, gameArtURLPrefix) {
		return nil
	}
	name := filepath.Base(strings.TrimPrefix(artURL, gameArtURLPrefix))
	if name == "" || name == "." || name == ".." {
		return nil
	}

	dir, err := GameArtDir()
	if err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(dir, name))
	return nil
}
