package utils

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"yukihub/internal/version"
)

const (
	bangumiClientIDEnv        = "YUKIHUB_BANGUMI_CLIENT_ID"
	bangumiClientSecretEnv    = "YUKIHUB_BANGUMI_CLIENT_SECRET"
	hikarinagiClientIDEnv     = "YUKIHUB_HIKARINAGI_CLIENT_ID"
	hikarinagiClientSecretEnv = "YUKIHUB_HIKARINAGI_CLIENT_SECRET"
	hikarinagiScopesEnv       = "YUKIHUB_HIKARINAGI_SCOPES"
	touchGalTokenEnv          = "YUKIHUB_TOUCHGAL_TOKEN"
	umbraClientIDEnv          = "YUKIHUB_UMBRA_CLIENT_ID"
	umbraRegistrationTokenEnv = "YUKIHUB_UMBRA_REGISTRATION_TOKEN"
)

func LoadEnvFilesIfExists(filenames ...string) error {
	existingFiles := make([]string, 0, len(filenames))
	for _, filename := range filenames {
		if _, err := os.Stat(filename); err == nil {
			existingFiles = append(existingFiles, filename)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("检查 env 文件 %s 失败: %w", filename, err)
		}
	}
	if len(existingFiles) == 0 {
		return nil
	}
	if err := godotenv.Load(existingFiles...); err != nil {
		return fmt.Errorf("加载 env 文件失败: %w", err)
	}
	return nil
}

// ApplyDevBuildEnvFallbacks makes build-time credentials available to wails dev.
// Real build-time ldflags keep priority; environment variables only fill blanks.
func ApplyDevBuildEnvFallbacks() {
	if strings.TrimSpace(version.BangumiOAuthClientID) == "" {
		version.BangumiOAuthClientID = strings.TrimSpace(os.Getenv(bangumiClientIDEnv))
	}
	if strings.TrimSpace(version.BangumiOAuthClientSecret) == "" {
		version.BangumiOAuthClientSecret = strings.TrimSpace(os.Getenv(bangumiClientSecretEnv))
	}
	if strings.TrimSpace(version.HikarinagiOAuthClientID) == "" {
		version.HikarinagiOAuthClientID = strings.TrimSpace(os.Getenv(hikarinagiClientIDEnv))
	}
	if strings.TrimSpace(version.HikarinagiOAuthClientSecret) == "" {
		version.HikarinagiOAuthClientSecret = strings.TrimSpace(os.Getenv(hikarinagiClientSecretEnv))
	}
	if strings.TrimSpace(version.HikarinagiOAuthScopes) == "" {
		version.HikarinagiOAuthScopes = strings.TrimSpace(os.Getenv(hikarinagiScopesEnv))
	}
	if strings.TrimSpace(version.TouchGalAPIToken) == "" {
		version.TouchGalAPIToken = strings.TrimSpace(os.Getenv(touchGalTokenEnv))
	}
	if strings.TrimSpace(version.UmbraOAuthClientID) == "" {
		version.UmbraOAuthClientID = strings.TrimSpace(os.Getenv(umbraClientIDEnv))
	}
	if strings.TrimSpace(version.UmbraRegistrationToken) == "" {
		version.UmbraRegistrationToken = strings.TrimSpace(os.Getenv(umbraRegistrationTokenEnv))
	}
}
