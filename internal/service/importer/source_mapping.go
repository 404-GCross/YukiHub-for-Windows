package importer

import (
	"strings"

	"yukihub/internal/common/enums"
)

// mapExternalSourceName 把外部导入格式里的**来源名**映射成桌面端的来源枚举。
//
// 全仓库只保留这一份实现，YukiHub 备份与 Playnite 库共用。
// 这类「按名字映射」的表最容易漏项，而漏项的后果不是报错，而是
// **静默把来源变成 local 或另一个来源**：
//
//   - YukiHub 备份：手机版 `settings.metadata_source` 可以是 nextmoe，
//     而这份表曾漏了 nextmoe → 映射成 local → 偏好来源判定被跳过 →
//     最终按兜底优先级落到 vndb（用户看到「nextmoe 变成 vndb」）
//   - Playnite 库：来源名是用户自由填写的字符串，之前只认 4 个名字，
//     其余（含桌面端自己支持的 hikarinagi / nextmoe / bangumi_mirror）静默变 local
//
// 新增来源时只改这里，不要再在调用方各写一份 switch。
func mapExternalSourceName(source string) enums.SourceType {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case string(enums.VNDB):
		return enums.VNDB
	// bangumi_mirror 与主站共用一套 API 与 subject id，且手机版**从不把它写成
	// 缓存来源**（缓存里恒为 "bangumi"，只有设置项 metadata_source 会出现
	// bangumi_mirror）。因此这里折叠成 Bangumi：这样备份里的偏好来源
	// "bangumi_mirror" 才能匹配上缓存条目里的 "bangumi"。
	case string(enums.Bangumi), string(enums.BangumiMirror):
		return enums.Bangumi
	case string(enums.Ymgal):
		return enums.Ymgal
	case string(enums.Hikarinagi):
		return enums.Hikarinagi
	case string(enums.NextMoe):
		return enums.NextMoe
	case string(enums.Steam):
		return enums.Steam
	case string(enums.DLsite):
		return enums.DLsite
	case string(enums.TouchGal):
		return enums.TouchGal
	case string(enums.ErogameScape):
		return enums.ErogameScape
	default:
		return enums.Local
	}
}
