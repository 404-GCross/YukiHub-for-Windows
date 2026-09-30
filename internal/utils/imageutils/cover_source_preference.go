package imageutils

import (
	"yukihub/internal/utils/coverutils"
)

// CoverSourcePreferenceProvider 提供「封面源优先度」配置，由 appconf.AppConfig 实现。
//
// 用接口而不是直接依赖 appconf，是为了让 imageutils 保持纯工具层，
// 同时又能拿到用户设置：谁把配置传进来，谁就实现这个方法。
type CoverSourcePreferenceProvider interface {
	CoverSourcePreference() coverutils.Preference
}

// resolvePreferredCoverURL 按封面源偏好改写远程封面地址。
//
// 识别不出来源（例如 hikarinagi / ymgal 的地址）或不满足改写条件时原样返回。
func resolvePreferredCoverURL(provider CoverSourcePreferenceProvider, imageURL string) string {
	if provider == nil {
		return imageURL
	}
	preference := provider.CoverSourcePreference()
	if preference == coverutils.OriginalPreference {
		return imageURL
	}
	return coverutils.ResolveURLByHost(preference, imageURL)
}

// coverSourcePreferenceProvider 从一个只承诺代理配置的提供者里尽量取出封面源偏好。
// 取不到时返回 nil，调用方按「不改写」处理。
func coverSourcePreferenceProvider(provider interface{}) CoverSourcePreferenceProvider {
	if provider == nil {
		return nil
	}
	if resolved, ok := provider.(CoverSourcePreferenceProvider); ok {
		return resolved
	}
	return nil
}
