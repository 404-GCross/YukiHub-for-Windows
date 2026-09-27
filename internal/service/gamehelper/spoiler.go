package gamehelper

import (
	"strings"

	"yukihub/internal/appconf"
	"yukihub/internal/common/vo"
)

func NormalizeSpoilerLevel(level string) string {
	normalized := strings.ToLower(strings.TrimSpace(level))
	if normalized == "" {
		return "none"
	}
	return normalized
}

func BuildSpoilerContext(config *appconf.AppConfig) vo.SpoilerContext {
	if config == nil {
		return vo.SpoilerContext{GlobalLevel: "none"}
	}

	return vo.SpoilerContext{
		GlobalLevel: NormalizeSpoilerLevel(config.AISpoilerLevel),
	}
}
