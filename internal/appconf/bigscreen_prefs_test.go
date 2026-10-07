package appconf

import "testing"

// 老配置（写在本轮新增字段之前）反序列化时不会带上这些键，
// 必须保留 defaultAppConfig 给的默认值 —— 升级后大屏行为不能变。
func TestBigScreenPreferencesDefaultValues(t *testing.T) {
	config := defaultAppConfig()

	if config.BigScreenSoundVolume != DefaultBigScreenSoundVolume {
		t.Errorf("sound_volume 默认值 = %d, want %d", config.BigScreenSoundVolume, DefaultBigScreenSoundVolume)
	}
	if !config.BigScreenFocusTicks {
		t.Error("focus_ticks 默认应为 true")
	}
	if !config.BigScreenIntroEnabled {
		t.Error("intro_enabled 默认应为 true")
	}
	if config.BigScreenShowTitles {
		t.Error("show_titles 默认应为 false（游戏名已在信息浮层展示）")
	}
	if config.BigScreenCardScale != DefaultBigScreenCardScale {
		t.Errorf("card_scale 默认值 = %d, want %d", config.BigScreenCardScale, DefaultBigScreenCardScale)
	}
	if config.BigScreenFocusScale != DefaultBigScreenFocusScale {
		t.Errorf("focus_scale 默认值 = %d, want %d", config.BigScreenFocusScale, DefaultBigScreenFocusScale)
	}
	if config.BigScreenKeyStyle != DefaultBigScreenKeyStyle {
		t.Errorf("key_style 默认值 = %q, want %q", config.BigScreenKeyStyle, DefaultBigScreenKeyStyle)
	}
	if config.BigScreenHintMode != DefaultBigScreenHintMode {
		t.Errorf("hint_mode 默认值 = %q, want %q", config.BigScreenHintMode, DefaultBigScreenHintMode)
	}
	if config.BigScreenRailExpanded {
		t.Error("rail_expanded 默认应为 false")
	}
	if !config.BigScreenTrailerEnabled {
		t.Error("trailer_enabled 默认应为 true")
	}
	// 手机端 M18 之后的默认值：PV 不静音（旧默认 true 让用户以为"PV 没声音"）。
	if config.BigScreenTrailerMuted {
		t.Error("trailer_muted 默认应为 false")
	}
	if config.BigScreenTrailerDelayMs != DefaultBigScreenTrailerDelayMs {
		t.Errorf("trailer_delay_ms 默认值 = %d, want %d", config.BigScreenTrailerDelayMs, DefaultBigScreenTrailerDelayMs)
	}
	// 手机端默认铺满裁切（fit=false）。
	if config.BigScreenPVFit {
		t.Error("pv_fit 默认应为 false（铺满裁切）")
	}
	if !config.BigScreenPVScrim {
		t.Error("pv_scrim 默认应为 true")
	}
	if config.BigScreenPVScrimPercent != DefaultBigScreenPVScrimPercent {
		t.Errorf("pv_scrim_percent 默认值 = %d, want %d", config.BigScreenPVScrimPercent, DefaultBigScreenPVScrimPercent)
	}
}

func TestNormalizeBigScreenPreferences(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*AppConfig)
		assert func(*testing.T, *AppConfig)
	}{
		{
			name:   "枚举项白名单收敛",
			mutate: func(c *AppConfig) { c.BigScreenKeyStyle = "  PS  " },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenKeyStyle != "ps" {
					t.Errorf("key_style = %q, want ps", c.BigScreenKeyStyle)
				}
			},
		},
		{
			name:   "未知按键风格回默认",
			mutate: func(c *AppConfig) { c.BigScreenKeyStyle = "switch" },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenKeyStyle != DefaultBigScreenKeyStyle {
					t.Errorf("key_style = %q, want %q", c.BigScreenKeyStyle, DefaultBigScreenKeyStyle)
				}
			},
		},
		{
			name:   "未知提示条模式回 auto",
			mutate: func(c *AppConfig) { c.BigScreenHintMode = "sometimes" },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenHintMode != DefaultBigScreenHintMode {
					t.Errorf("hint_mode = %q, want %q", c.BigScreenHintMode, DefaultBigScreenHintMode)
				}
			},
		},
		{
			// card_scale 为 0 会让卡片宽度算成 0，整个货架消失。
			name:   "卡片倍率越界夹回",
			mutate: func(c *AppConfig) { c.BigScreenCardScale = 0 },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenCardScale != MinBigScreenCardScale {
					t.Errorf("card_scale = %d, want %d", c.BigScreenCardScale, MinBigScreenCardScale)
				}
			},
		},
		{
			name:   "卡片倍率上限夹回",
			mutate: func(c *AppConfig) { c.BigScreenCardScale = 999 },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenCardScale != MaxBigScreenCardScale {
					t.Errorf("card_scale = %d, want %d", c.BigScreenCardScale, MaxBigScreenCardScale)
				}
			},
		},
		{
			name:   "焦点缩放允许 0（只描边）",
			mutate: func(c *AppConfig) { c.BigScreenFocusScale = -20 },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenFocusScale != 0 {
					t.Errorf("focus_scale = %d, want 0", c.BigScreenFocusScale)
				}
			},
		},
		{
			name:   "音量与遮罩强度夹进 0-100",
			mutate: func(c *AppConfig) { c.BigScreenSoundVolume = 150; c.BigScreenPVScrimPercent = -5 },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenSoundVolume != 100 {
					t.Errorf("sound_volume = %d, want 100", c.BigScreenSoundVolume)
				}
				if c.BigScreenPVScrimPercent != 0 {
					t.Errorf("pv_scrim_percent = %d, want 0", c.BigScreenPVScrimPercent)
				}
			},
		},
		{
			name:   "预告片延迟夹进 300-5000",
			mutate: func(c *AppConfig) { c.BigScreenTrailerDelayMs = 50 },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenTrailerDelayMs != MinBigScreenTrailerDelayMs {
					t.Errorf("trailer_delay_ms = %d, want %d", c.BigScreenTrailerDelayMs, MinBigScreenTrailerDelayMs)
				}
			},
		},
		{
			name:   "正常值原样保留",
			mutate: func(c *AppConfig) { c.BigScreenCardScale = 128; c.BigScreenTrailerDelayMs = 3000 },
			assert: func(t *testing.T, c *AppConfig) {
				if c.BigScreenCardScale != 128 || c.BigScreenTrailerDelayMs != 3000 {
					t.Errorf("值被误改: card_scale=%d delay=%d", c.BigScreenCardScale, c.BigScreenTrailerDelayMs)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := defaultAppConfig()
			tc.mutate(config)
			NormalizeBigScreenPreferences(config)
			tc.assert(t, config)
		})
	}
}

// nil 配置不能被归一化函数打崩（SaveConfig 会对拷贝调用）。
func TestNormalizeBigScreenPreferencesNilSafe(t *testing.T) {
	NormalizeBigScreenPreferences(nil)
}
