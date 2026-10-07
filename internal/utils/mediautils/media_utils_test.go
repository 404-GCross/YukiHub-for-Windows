package mediautils

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempTrailer 在临时目录里造一个假视频文件，供复制逻辑使用。
func writeTempTrailer(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("fake video bytes"), 0644); err != nil {
		t.Fatalf("创建源文件失败: %v", err)
	}
	return path
}

func TestTrailerExtensionsReturnsCopy(t *testing.T) {
	extensions := TrailerExtensions()
	if len(extensions) == 0 {
		t.Fatal("扩展名列表不应为空")
	}

	extensions[0] = ".tampered"
	if TrailerExtensions()[0] == ".tampered" {
		t.Fatal("TrailerExtensions 应返回副本，外部修改不得污染内部列表")
	}
}

func TestIsSupportedTrailerExt(t *testing.T) {
	cases := []struct {
		ext      string
		expected bool
	}{
		{".mp4", true},
		{".MP4", true},
		{" .webm ", true},
		{".mkv", true},
		{".avi", true},
		{".txt", false},
		{".png", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := IsSupportedTrailerExt(tc.ext); got != tc.expected {
			t.Errorf("IsSupportedTrailerExt(%q) = %v, 期望 %v", tc.ext, got, tc.expected)
		}
	}
}

func TestSaveTrailerCopiesIntoManagedDir(t *testing.T) {
	dir := t.TempDir()
	restore := SetTrailersDirForTest(dir)
	defer restore()

	src := writeTempTrailer(t, "demo.mp4")

	got, err := SaveTrailer(src, "game-001")
	if err != nil {
		t.Fatalf("SaveTrailer 失败: %v", err)
	}
	if got != "/local/trailers/game-001.mp4" {
		t.Fatalf("返回路径不匹配: %s", got)
	}

	content, err := os.ReadFile(filepath.Join(dir, "game-001.mp4"))
	if err != nil {
		t.Fatalf("复制后的文件不存在: %v", err)
	}
	if string(content) != "fake video bytes" {
		t.Errorf("文件内容不匹配: %s", string(content))
	}
}

func TestSaveTrailerReplacesPreviousFileOnFormatChange(t *testing.T) {
	dir := t.TempDir()
	restore := SetTrailersDirForTest(dir)
	defer restore()

	if _, err := SaveTrailer(writeTempTrailer(t, "first.mp4"), "game-002"); err != nil {
		t.Fatalf("首次 SaveTrailer 失败: %v", err)
	}

	got, err := SaveTrailer(writeTempTrailer(t, "second.webm"), "game-002")
	if err != nil {
		t.Fatalf("二次 SaveTrailer 失败: %v", err)
	}
	if got != "/local/trailers/game-002.webm" {
		t.Fatalf("返回路径不匹配: %s", got)
	}

	if _, err := os.Stat(filepath.Join(dir, "game-002.mp4")); !os.IsNotExist(err) {
		t.Error("换格式重选后旧文件应被清理")
	}
	if _, err := os.Stat(filepath.Join(dir, "game-002.webm")); err != nil {
		t.Errorf("新文件应存在: %v", err)
	}
}

func TestSaveTrailerRejectsUnsupportedOrIncompleteInput(t *testing.T) {
	dir := t.TempDir()
	restore := SetTrailersDirForTest(dir)
	defer restore()

	if _, err := SaveTrailer(writeTempTrailer(t, "demo.txt"), "game-003"); err == nil {
		t.Error("不支持的扩展名应报错")
	}
	if _, err := SaveTrailer(writeTempTrailer(t, "demo.mp4"), ""); err == nil {
		t.Error("缺少 gameID 应报错")
	}
	if _, err := SaveTrailer(filepath.Join(t.TempDir(), "missing.mp4"), "game-003"); err == nil {
		t.Error("源文件不存在应报错")
	}
}

func TestRemoveTrailerRemovesAllKnownExtensions(t *testing.T) {
	dir := t.TempDir()
	restore := SetTrailersDirForTest(dir)
	defer restore()

	for _, ext := range []string{".mp4", ".webm"} {
		if err := os.WriteFile(filepath.Join(dir, "game-004"+ext), []byte("x"), 0644); err != nil {
			t.Fatalf("准备文件失败: %v", err)
		}
	}
	// 应保留同目录下其他游戏的文件
	if err := os.WriteFile(filepath.Join(dir, "game-005.mp4"), []byte("x"), 0644); err != nil {
		t.Fatalf("准备文件失败: %v", err)
	}

	if err := RemoveTrailer("game-004"); err != nil {
		t.Fatalf("RemoveTrailer 失败: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	remaining := make([]string, 0, len(entries))
	for _, entry := range entries {
		remaining = append(remaining, entry.Name())
	}
	if len(remaining) != 1 || remaining[0] != "game-005.mp4" {
		t.Errorf("删除结果不符合预期: %v", remaining)
	}

	if err := RemoveTrailer("game-006"); err != nil {
		t.Errorf("删除不存在的预告片应静默成功: %v", err)
	}
}

func TestSaveIntroVideoUsesDedicatedDirAndReplacesPrevious(t *testing.T) {
	dir := t.TempDir()
	restore := SetTrailersDirForTest(dir)
	defer restore()

	got, err := SaveIntroVideo(writeTempTrailer(t, "opening.mp4"))
	if err != nil {
		t.Fatalf("SaveIntroVideo 失败: %v", err)
	}
	if got != "/local/intro/intro.mp4" {
		t.Fatalf("返回路径不匹配: %s", got)
	}

	// 入场视频放在独立目录里，不能污染 trailers 目录（那里是按 gameID 命名的）
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取预告片目录失败: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("trailers 目录不应被写入: %v", entries)
	}

	// 换格式重选：旧文件清掉，同时只保留一个入场视频
	got, err = SaveIntroVideo(writeTempTrailer(t, "opening.webm"))
	if err != nil {
		t.Fatalf("二次 SaveIntroVideo 失败: %v", err)
	}
	if got != "/local/intro/intro.webm" {
		t.Fatalf("返回路径不匹配: %s", got)
	}
	introDir, err := IntroVideoDir()
	if err != nil {
		t.Fatalf("IntroVideoDir 失败: %v", err)
	}
	introEntries, err := os.ReadDir(introDir)
	if err != nil {
		t.Fatalf("读取入场视频目录失败: %v", err)
	}
	if len(introEntries) != 1 || introEntries[0].Name() != "intro.webm" {
		t.Errorf("入场视频目录应只剩一个新文件: %v", introEntries)
	}
}

func TestSaveIntroVideoRejectsUnsupportedAndRemoveIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	restore := SetTrailersDirForTest(dir)
	defer restore()

	if _, err := SaveIntroVideo(writeTempTrailer(t, "opening.txt")); err == nil {
		t.Error("不支持的扩展名应报错")
	}
	if _, err := SaveIntroVideo(""); err == nil {
		t.Error("空路径应报错")
	}

	if _, err := SaveIntroVideo(writeTempTrailer(t, "opening.mp4")); err != nil {
		t.Fatalf("SaveIntroVideo 失败: %v", err)
	}
	if err := RemoveIntroVideo(); err != nil {
		t.Fatalf("RemoveIntroVideo 失败: %v", err)
	}
	introDir, err := IntroVideoDir()
	if err != nil {
		t.Fatalf("IntroVideoDir 失败: %v", err)
	}
	introEntries, err := os.ReadDir(introDir)
	if err != nil {
		t.Fatalf("读取入场视频目录失败: %v", err)
	}
	if len(introEntries) != 0 {
		t.Errorf("删除后目录应为空: %v", introEntries)
	}
	if err := RemoveIntroVideo(); err != nil {
		t.Errorf("删除不存在的入场视频应静默成功: %v", err)
	}
}
