package appconf

import (
	"strings"
	"time"

	enums2 "yukihub/internal/common/enums"
	"yukihub/internal/utils/proxyutils"
)

func NormalizeScheduledDBBackup(config *AppConfig) bool {
	if config == nil {
		return false
	}

	changed := false
	mode := strings.ToLower(strings.TrimSpace(config.ScheduledDBBackupMode))
	if mode != ScheduledDBBackupModeDaily {
		mode = ScheduledDBBackupModeInterval
	}
	if config.ScheduledDBBackupMode != mode {
		config.ScheduledDBBackupMode = mode
		changed = true
	}

	interval := config.ScheduledDBBackupIntervalMinutes
	if interval < MinScheduledDBBackupIntervalMinutes {
		interval = DefaultScheduledDBBackupIntervalMinutes
	}
	if interval > MaxScheduledDBBackupIntervalMinutes {
		interval = MaxScheduledDBBackupIntervalMinutes
	}
	if config.ScheduledDBBackupIntervalMinutes != interval {
		config.ScheduledDBBackupIntervalMinutes = interval
		changed = true
	}

	backupTime := strings.TrimSpace(config.ScheduledDBBackupTime)
	if _, err := time.Parse("15:04", backupTime); err != nil {
		backupTime = DefaultScheduledDBBackupTime
	}
	if config.ScheduledDBBackupTime != backupTime {
		config.ScheduledDBBackupTime = backupTime
		changed = true
	}

	return changed
}

func NormalizeScrapedTagLimit(limit int) int {
	if limit < -1 {
		return -1
	}
	return limit
}

func NormalizeLocalDBBackupRetention(retention int) int {
	if retention < 1 {
		return DefaultLocalDBBackupRetention
	}
	return retention
}

func NormalizeHomeGameCarouselIntervalSec(intervalSec int) int {
	if intervalSec <= 0 {
		return DefaultHomeGameCarouselIntervalSec
	}
	if intervalSec < MinHomeGameCarouselIntervalSec {
		return MinHomeGameCarouselIntervalSec
	}
	return intervalSec
}

func NormalizeProcessDetectionTimeoutSec(timeoutSec int) int {
	if timeoutSec <= 0 {
		return DefaultProcessDetectionTimeoutSec
	}
	if timeoutSec < MinProcessDetectionTimeoutSec {
		return MinProcessDetectionTimeoutSec
	}
	if timeoutSec > MaxProcessDetectionTimeoutSec {
		return MaxProcessDetectionTimeoutSec
	}
	return timeoutSec
}

// NormalizePlayTimingMode 把计时模式收敛到白名单。
//
// 空串（老配置里没有这个字段 / 反序列化缺省）与任何未知取值都回退到默认的
// 「进程监测」，保证升级用户行为不变。
func NormalizePlayTimingMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case PlayTimingModeManual:
		return PlayTimingModeManual
	default:
		return PlayTimingModeProcess
	}
}

func NormalizeBatchImportPreferences(config *AppConfig) bool {
	if config == nil {
		return false
	}

	changed := false
	switch config.BatchImportScanPreset {
	case "scan_parent", "scan_library_child", "hierarchy_child":
	default:
		config.BatchImportScanPreset = DefaultBatchImportScanPreset
		changed = true
	}

	if config.BatchImportHierarchyDepth < 0 {
		config.BatchImportHierarchyDepth = 0
		changed = true
	}
	if config.BatchImportHierarchyDepth > MaxBatchImportHierarchyDepth {
		config.BatchImportHierarchyDepth = MaxBatchImportHierarchyDepth
		changed = true
	}

	if config.BatchImportPreferredSource != "" {
		if _, ok := allowedMetadataSourceSet[config.BatchImportPreferredSource]; !ok {
			config.BatchImportPreferredSource = ""
			changed = true
		}
	}

	return changed
}

func normalizeMetadataSources(sources []string) []string {
	if len(sources) == 0 {
		return cloneStringSlice(defaultMetadataSources)
	}

	result := make([]string, 0, len(defaultMetadataSources))
	seen := make(map[string]struct{}, len(defaultMetadataSources))

	for _, source := range sources {
		normalized := strings.ToLower(strings.TrimSpace(source))
		if normalized == "" {
			continue
		}
		if _, ok := allowedMetadataSourceSet[normalized]; !ok {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	if len(result) == 0 {
		return cloneStringSlice(defaultMetadataSources)
	}
	return result
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func NormalizeMCPPort(port int) int {
	if port < 1 || port > 65535 {
		return DefaultMCPPort
	}
	return port
}

func NormalizeGameCardLayout(layout string) string {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "landscape":
		return "landscape"
	default:
		return DefaultGameCardLayout
	}
}

func NormalizeBigScreenDefaultCategory(category string) string {
	trimmed := strings.TrimSpace(category)
	switch trimmed {
	case "all", "favorites", "recent", "playing", "completed", "unplayed":
		return trimmed
	default:
		return DefaultBigScreenDefaultCategory
	}
}

func NormalizeBigScreenEffectLevel(level string) string {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "off", "low", "high":
		return normalized
	default:
		return DefaultBigScreenEffectLevel
	}
}

// NormalizeBigScreenKeyStyle 白名单收敛按键图标风格，识别不了的一律回 Xbox。
func NormalizeBigScreenKeyStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "ps":
		return "ps"
	default:
		return DefaultBigScreenKeyStyle
	}
}

// NormalizeBigScreenHintMode 收敛按键提示条模式：auto（自动淡出）/ always / off。
func NormalizeBigScreenHintMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "always", "off":
		return normalized
	default:
		return DefaultBigScreenHintMode
	}
}

// NormalizeBigScreenCardScale 把卡片大小倍率（×100）夹进可辨识区间。
func NormalizeBigScreenCardScale(scale int) int {
	return clampInt(scale, MinBigScreenCardScale, MaxBigScreenCardScale)
}

// NormalizeBigScreenFocusScale 把焦点缩放幅度夹进 0–150（0 = 只描边不缩放）。
func NormalizeBigScreenFocusScale(scale int) int {
	return clampInt(scale, 0, MaxBigScreenFocusScale)
}

func NormalizeBigScreenSoundVolume(volume int) int {
	return clampInt(volume, 0, 100)
}

func NormalizeBigScreenTrailerDelayMs(delay int) int {
	return clampInt(delay, MinBigScreenTrailerDelayMs, MaxBigScreenTrailerDelayMs)
}

func NormalizeBigScreenPVScrimPercent(percent int) int {
	return clampInt(percent, 0, 100)
}

// NormalizeBigScreenPreferences 一次性收敛大屏模式的全部偏好。
//
// 数值项用夹取、枚举项用白名单：老配置缺字段时反序列化会保留默认值，
// 真正需要修的只有人为改坏或历史遗留的越界值。用户改坏配置文件后
// 大屏不会因此进不去（例如 card_scale=0 会让卡片宽度算成 0）。
func NormalizeBigScreenPreferences(config *AppConfig) {
	if config == nil {
		return
	}
	config.BigScreenDefaultCategory = NormalizeBigScreenDefaultCategory(config.BigScreenDefaultCategory)
	config.BigScreenEffectLevel = NormalizeBigScreenEffectLevel(config.BigScreenEffectLevel)
	config.BigScreenKeyStyle = NormalizeBigScreenKeyStyle(config.BigScreenKeyStyle)
	config.BigScreenHintMode = NormalizeBigScreenHintMode(config.BigScreenHintMode)
	config.BigScreenCardScale = NormalizeBigScreenCardScale(config.BigScreenCardScale)
	config.BigScreenFocusScale = NormalizeBigScreenFocusScale(config.BigScreenFocusScale)
	config.BigScreenSoundVolume = NormalizeBigScreenSoundVolume(config.BigScreenSoundVolume)
	config.BigScreenTrailerDelayMs = NormalizeBigScreenTrailerDelayMs(config.BigScreenTrailerDelayMs)
	config.BigScreenPVScrimPercent = NormalizeBigScreenPVScrimPercent(config.BigScreenPVScrimPercent)
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func NormalizeMetadataCoverSource(source enums2.MetadataCoverSource) enums2.MetadataCoverSource {
	switch strings.ToLower(strings.TrimSpace(string(source))) {
	case string(enums2.MetadataCoverSourceOriginal):
		return enums2.MetadataCoverSourceOriginal
	default:
		return enums2.MetadataCoverSourceHikarinagi
	}
}

func NormalizeMetadataCoverSources(config *AppConfig) {
	if config == nil {
		return
	}
	config.BangumiCoverSource = NormalizeMetadataCoverSource(config.BangumiCoverSource)
	config.VNDBCoverSource = NormalizeMetadataCoverSource(config.VNDBCoverSource)
}

// NormalizeCurrentMetadataSource 校验「当前资料源」，非法取值回落到默认 VNDB。
//
// 只认「可作为资料源开启」的那几个（allowedMetadataSourceSet，与设置页下拉一致）。
// IsSelectableMetadataSource 判断某个资料源是否属于「当前资料源」的白名单。
//
// 这张表既是设置页给用户的可选项，也正好等于手机版 importSnapshot 落回
// settings.metadata_source 时接受的那六个值（vndb / bangumi / bangumi_mirror /
// ymgal / hikarinagi / nextmoe）。导入同步快照时用它校验，避免把桌面端独有的
// 来源（steam / dlsite / touchgal / erogamescape）当成跨端全局偏好写进配置 ——
// 那些值手机端根本不认识，写过去会被它忽略。
func IsSelectableMetadataSource(source string) bool {
	_, ok := allowedMetadataSourceSet[strings.ToLower(strings.TrimSpace(source))]
	return ok
}

func NormalizeCurrentMetadataSource(config *AppConfig) bool {
	if config == nil {
		return false
	}
	normalized := enums2.SourceType(strings.ToLower(strings.TrimSpace(string(config.CurrentMetadataSource))))
	if _, ok := allowedMetadataSourceSet[string(normalized)]; !ok {
		normalized = DefaultCurrentMetadataSource
	}
	if config.CurrentMetadataSource == normalized {
		return false
	}
	config.CurrentMetadataSource = normalized
	return true
}

func NormalizeProxySettings(config *AppConfig) bool {
	if config == nil {
		return false
	}

	changed := false
	normalizeMode := func(value string) string {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case proxyutils.ProxyModeManual:
			return proxyutils.ProxyModeManual
		case proxyutils.ProxyModeDirect:
			return proxyutils.ProxyModeDirect
		default:
			return proxyutils.ProxyModeSystem
		}
	}
	setMode := func(target *string) {
		next := normalizeMode(*target)
		if *target != next {
			*target = next
			changed = true
		}
	}

	trimmedProxyURL := strings.TrimSpace(config.NetworkProxyURL)
	if config.NetworkProxyURL != trimmedProxyURL {
		config.NetworkProxyURL = trimmedProxyURL
		changed = true
	}

	setMode(&config.NetworkProxyMode)

	return changed
}
