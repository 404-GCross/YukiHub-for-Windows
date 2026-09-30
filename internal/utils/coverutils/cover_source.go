// Package coverutils 统一处理「封面源优先度」：把国内网络常常直连不通的
// 原始封面地址（Bangumi / VNDB）换成 hikarinagi 提供的镜像地址。
//
// 手机版在下载出口无条件做这件事（MetadataUtils.proxyBangumiImage），
// 桌面端把它做成可配置项（设置 → 元数据 → Bangumi / VNDB 封面源），
// 并且必须作用在**所有**出口上——刮削、备份导入、旧缓存、图片代理——
// 否则绕过刮削流程的地址（例如从手机版备份导进来的原始 vndb / bgm 地址）
// 会照旧直连失败，表现为「封面空白 + 每次切页都重新加载」。
package coverutils

import (
	"net/url"
	"strings"

	"yukihub/internal/common/enums"
)

const (
	// HikarinagiBangumiImageProxyBaseURL 是 Bangumi 封面图的镜像前缀，
	// 用法是把原始地址整段接在后面。
	HikarinagiBangumiImageProxyBaseURL = "https://imagesp.yurari.moe/bangumi/"
	// HikarinagiVNDBImageProxyBaseURL 是 VNDB 封面图的镜像前缀。
	HikarinagiVNDBImageProxyBaseURL = "https://imagesp.yurari.moe/vndb/"
)

// Preference 描述两个元数据来源各自取用哪个封面源。
type Preference struct {
	Bangumi enums.MetadataCoverSource
	VNDB    enums.MetadataCoverSource
}

// OriginalPreference 表示完全不做改写。
var OriginalPreference = Preference{
	Bangumi: enums.MetadataCoverSourceOriginal,
	VNDB:    enums.MetadataCoverSourceOriginal,
}

// DetectSource 从地址的主机名推断封面所属来源，识别不出来时返回空串。
//
// 只认 bgm.tv / vndb.org 这两个主域及其子域，避免误伤形如 notbgm.tv 的地址。
func DetectSource(rawURL string) enums.SourceType {
	host := hostOf(rawURL)
	switch {
	case host == "bgm.tv" || strings.HasSuffix(host, ".bgm.tv"):
		return enums.Bangumi
	case host == "vndb.org" || strings.HasSuffix(host, ".vndb.org"):
		return enums.VNDB
	default:
		return ""
	}
}

// ResolveURL 按来源与偏好改写封面地址；不需要改写时原样返回。
func ResolveURL(pref Preference, source enums.SourceType, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	baseURL := ""
	switch source {
	case enums.Bangumi, enums.BangumiMirror:
		if pref.Bangumi != enums.MetadataCoverSourceHikarinagi {
			return rawURL
		}
		baseURL = HikarinagiBangumiImageProxyBaseURL
	case enums.VNDB:
		if pref.VNDB != enums.MetadataCoverSourceHikarinagi {
			return rawURL
		}
		baseURL = HikarinagiVNDBImageProxyBaseURL
	default:
		return rawURL
	}

	if IsProxiedURL(rawURL) {
		return rawURL
	}
	return baseURL + rawURL
}

// ResolveURLByHost 与 ResolveURL 相同，但由地址本身推断来源。
// 用于只剩地址、已经拿不到来源信息的场景（备份导入、旧缓存）。
func ResolveURLByHost(pref Preference, rawURL string) string {
	return ResolveURL(pref, DetectSource(rawURL), rawURL)
}

// IsProxiedURL 判断地址是否已经指向镜像，避免重复包裹。
func IsProxiedURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	return strings.HasPrefix(trimmed, HikarinagiBangumiImageProxyBaseURL) ||
		strings.HasPrefix(trimmed, HikarinagiVNDBImageProxyBaseURL)
}

func hostOf(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
