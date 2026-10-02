package appconf

import "strings"

func SanitizeUmbraConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	baseURL := strings.TrimRight(strings.TrimSpace(config.UmbraBaseURL), "/")
	changed := config.UmbraBaseURL != baseURL
	config.UmbraBaseURL = baseURL
	if baseURL == "" && config.UmbraAuthenticated {
		config.UmbraAuthenticated = false
		changed = true
	}
	return changed
}

func SanitizeOneDriveOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedClientID := strings.TrimSpace(config.OneDriveClientID)
	changed := config.OneDriveClientID != trimmedClientID
	config.OneDriveClientID = trimmedClientID

	if config.OneDriveClientID == legacyOneDriveDefaultClientID {
		config.OneDriveClientID = ""
		changed = true
		if config.OneDriveRefreshToken != "" {
			config.OneDriveRefreshToken = ""
		}
	}

	return changed
}

func SanitizeBangumiOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedAccessToken := strings.TrimSpace(config.BangumiAccessToken)
	trimmedRefreshToken := strings.TrimSpace(config.BangumiRefreshToken)
	trimmedExpiresAt := strings.TrimSpace(config.BangumiTokenExpiresAt)
	trimmedUserID := strings.TrimSpace(config.BangumiAuthorizedUserID)
	trimmedUsername := strings.TrimSpace(config.BangumiAuthorizedUsername)
	trimmedAvatarURL := strings.TrimSpace(config.BangumiAuthorizedAvatarURL)
	trimmedAuthError := strings.TrimSpace(config.BangumiAuthError)

	changed := config.BangumiAccessToken != trimmedAccessToken ||
		config.BangumiRefreshToken != trimmedRefreshToken ||
		config.BangumiTokenExpiresAt != trimmedExpiresAt ||
		config.BangumiAuthorizedUserID != trimmedUserID ||
		config.BangumiAuthorizedUsername != trimmedUsername ||
		config.BangumiAuthorizedAvatarURL != trimmedAvatarURL ||
		config.BangumiAuthError != trimmedAuthError

	config.BangumiAccessToken = trimmedAccessToken
	config.BangumiRefreshToken = trimmedRefreshToken
	config.BangumiTokenExpiresAt = trimmedExpiresAt
	config.BangumiAuthorizedUserID = trimmedUserID
	config.BangumiAuthorizedUsername = trimmedUsername
	config.BangumiAuthorizedAvatarURL = trimmedAvatarURL
	config.BangumiAuthError = trimmedAuthError
	if config.BangumiStatusPushEnabled == nil {
		config.BangumiStatusPushEnabled = boolPtr(true)
		changed = true
	}

	if config.BangumiAccessToken == "" && config.BangumiTokenExpiresAt != "" {
		config.BangumiTokenExpiresAt = ""
		changed = true
	}

	return changed
}

// SanitizeNextMoeOAuthConfig 修剪 NextMoe 令牌与账号标识，并在无访问令牌时清掉过期时间。
func SanitizeNextMoeOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedAccessToken := strings.TrimSpace(config.NextMoeAccessToken)
	trimmedRefreshToken := strings.TrimSpace(config.NextMoeRefreshToken)
	trimmedExpiresAt := strings.TrimSpace(config.NextMoeTokenExpiresAt)
	trimmedAccountLabel := strings.TrimSpace(config.NextMoeAccountLabel)

	changed := config.NextMoeAccessToken != trimmedAccessToken ||
		config.NextMoeRefreshToken != trimmedRefreshToken ||
		config.NextMoeTokenExpiresAt != trimmedExpiresAt ||
		config.NextMoeAccountLabel != trimmedAccountLabel

	config.NextMoeAccessToken = trimmedAccessToken
	config.NextMoeRefreshToken = trimmedRefreshToken
	config.NextMoeTokenExpiresAt = trimmedExpiresAt
	config.NextMoeAccountLabel = trimmedAccountLabel

	if config.NextMoeAccessToken == "" && config.NextMoeTokenExpiresAt != "" {
		config.NextMoeTokenExpiresAt = ""
		changed = true
	}

	return changed
}

func SanitizeHikarinagiOAuthConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	trimmedAccessToken := strings.TrimSpace(config.HikarinagiAccessToken)
	trimmedRefreshToken := strings.TrimSpace(config.HikarinagiRefreshToken)
	trimmedExpiresAt := strings.TrimSpace(config.HikarinagiTokenExpiresAt)
	trimmedUserID := strings.TrimSpace(config.HikarinagiAuthorizedUserID)
	trimmedUsername := strings.TrimSpace(config.HikarinagiAuthorizedUsername)
	trimmedAvatarURL := strings.TrimSpace(config.HikarinagiAuthorizedAvatarURL)
	trimmedAuthError := strings.TrimSpace(config.HikarinagiAuthError)

	changed := config.HikarinagiAccessToken != trimmedAccessToken ||
		config.HikarinagiRefreshToken != trimmedRefreshToken ||
		config.HikarinagiTokenExpiresAt != trimmedExpiresAt ||
		config.HikarinagiAuthorizedUserID != trimmedUserID ||
		config.HikarinagiAuthorizedUsername != trimmedUsername ||
		config.HikarinagiAuthorizedAvatarURL != trimmedAvatarURL ||
		config.HikarinagiAuthError != trimmedAuthError

	config.HikarinagiAccessToken = trimmedAccessToken
	config.HikarinagiRefreshToken = trimmedRefreshToken
	config.HikarinagiTokenExpiresAt = trimmedExpiresAt
	config.HikarinagiAuthorizedUserID = trimmedUserID
	config.HikarinagiAuthorizedUsername = trimmedUsername
	config.HikarinagiAuthorizedAvatarURL = trimmedAvatarURL
	config.HikarinagiAuthError = trimmedAuthError
	if config.HikarinagiStatusPushEnabled == nil {
		config.HikarinagiStatusPushEnabled = boolPtr(true)
		changed = true
	}

	if config.HikarinagiAccessToken == "" && config.HikarinagiTokenExpiresAt != "" {
		config.HikarinagiTokenExpiresAt = ""
		changed = true
	}

	return changed
}

// 账号令牌为空时（登出）一并清掉的字段。
func SanitizeYukiHubAccountConfig(config *AppConfig) bool {
	if config == nil {
		return false
	}

	changed := false
	trim := func(target *string) {
		trimmed := strings.TrimSpace(*target)
		if *target != trimmed {
			*target = trimmed
			changed = true
		}
	}

	trim(&config.YukiHubAccountAccessToken)
	trim(&config.YukiHubAccountRefreshToken)
	trim(&config.YukiHubAccountUserID)
	trim(&config.YukiHubAccountNickname)
	trim(&config.YukiHubAccountEmail)
	trim(&config.YukiHubAccountAvatar)
	trim(&config.LastYukiHubAccountSyncHash)
	trim(&config.LastYukiHubAccountSyncAt)

	// 没有访问令牌就不是「已登录」状态，身份字段一并清掉，
	// 否则界面会出现「未登录但显示昵称/头像」的中间态。
	if config.YukiHubAccountAccessToken == "" {
		if config.YukiHubAccountRefreshToken != "" {
			config.YukiHubAccountRefreshToken = ""
			changed = true
		}
		if config.YukiHubAccountUserID != "" {
			config.YukiHubAccountUserID = ""
			changed = true
		}
		if config.YukiHubAccountUID != 0 {
			config.YukiHubAccountUID = 0
			changed = true
		}
		if config.YukiHubAccountNickname != "" {
			config.YukiHubAccountNickname = ""
			changed = true
		}
		if config.YukiHubAccountAvatar != "" {
			config.YukiHubAccountAvatar = ""
			changed = true
		}
		config.YukiHubAccountKungalBound = false
		config.YukiHubAccountHikarinagiBound = false
	}

	return changed
}
