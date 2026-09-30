package gamehelper

import (
	"fmt"
	"strings"

	"yukihub/internal/common/enums"
	"yukihub/internal/models"
)

func IsSupportedMetadataSource(source enums.SourceType) bool {
	switch NormalizeMetadataSourceType(source) {
	case enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam, enums.DLsite,
		enums.TouchGal, enums.Hikarinagi, enums.ErogameScape,
		enums.BangumiMirror, enums.NextMoe:
		return true
	default:
		return false
	}
}

// NSFWAuthoritativeSources 返回「会给出可信 NSFW 标记」的元数据来源。
//
// 依据是各来源的解析实现：bangumi（含镜像站）/ vndb / hikarinagi / nextmoe 都会把
// 来源侧的 NSFW（或封面性感度）写进 `models.Game.IsNSFW`；ymgal / steam 等不会，
// 它们带回来的恒为 false —— 照抄反而会**清掉**用户已经在本地标好的 NSFW。
//
// 调用方（导入合并路径）请用这个函数生成判定，不要另外手写一份来源名单：
// 上游原本硬编码 `IN ('bangumi', 'vndb')`，新增 hikarinagi / nextmoe 后一直没跟上，
// 结果是这两个来源的游戏在合并导入时 NSFW 永远不会被更新。
func NSFWAuthoritativeSources() []enums.SourceType {
	return []enums.SourceType{
		enums.Bangumi,
		enums.BangumiMirror,
		enums.VNDB,
		enums.Hikarinagi,
		enums.NextMoe,
	}
}

// IsNSFWAuthoritativeSource 判断某个来源的 NSFW 标记是否可以直接采信。
func IsNSFWAuthoritativeSource(source enums.SourceType) bool {
	for _, candidate := range NSFWAuthoritativeSources() {
		if candidate == source {
			return true
		}
	}
	return false
}

func NormalizeMetadataSource(source enums.SourceType, sourceID string) (enums.SourceType, string, error) {
	source = NormalizeMetadataSourceType(source)
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return "", "", fmt.Errorf("元数据来源 ID 不能为空")
	}
	if !IsSupportedMetadataSource(source) {
		return "", "", fmt.Errorf("不支持的元数据来源: %s", source)
	}
	return source, sourceID, nil
}

func ValidateInitialMetadataSources(sources []models.GameMetadataSource) error {
	seen := make(map[enums.SourceType]struct{}, len(sources))
	for _, item := range sources {
		source, _, err := NormalizeMetadataSource(item.SourceType, item.SourceID)
		if err != nil {
			return err
		}
		if _, exists := seen[source]; exists {
			return fmt.Errorf("同一游戏的 %s 元数据记录存在多个，请移除错误的候选项", source)
		}
		seen[source] = struct{}{}
	}
	return nil
}

func DefaultMetadataSourceValue(source enums.SourceType) string {
	if source == "" {
		return string(enums.Local)
	}
	return string(source)
}

func NormalizeDefaultMetadataSource(source enums.SourceType, sourceID string) (enums.SourceType, string) {
	source = NormalizeMetadataSourceType(source)
	sourceID = strings.TrimSpace(sourceID)
	if source == "" || source == enums.Local {
		return enums.Local, ""
	}
	return source, sourceID
}
