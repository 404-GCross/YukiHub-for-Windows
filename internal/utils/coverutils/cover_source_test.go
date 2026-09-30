package coverutils

import (
	"testing"

	"yukihub/internal/common/enums"
)

func hikarinagiPreference() Preference {
	return Preference{
		Bangumi: enums.MetadataCoverSourceHikarinagi,
		VNDB:    enums.MetadataCoverSourceHikarinagi,
	}
}

func TestDetectSource(t *testing.T) {
	cases := map[string]enums.SourceType{
		"https://lain.bgm.tv/pic/cover/l/c4/85/207413_frAAo.jpg": enums.Bangumi,
		"https://bgm.tv/x.jpg":                 enums.Bangumi,
		"https://t.vndb.org/cv.t/61/77161.jpg": enums.VNDB,
		"https://vndb.org/x.jpg":               enums.VNDB,
		// 不能误伤形如 notbgm.tv 的地址
		"https://notbgm.tv/x.jpg":     "",
		"https://fakevndb.org/x.jpg":  "",
		"https://images.yurari.moe/x": "",
		"not a url":                   "",
		"":                            "",
	}

	for rawURL, want := range cases {
		if got := DetectSource(rawURL); got != want {
			t.Errorf("DetectSource(%q) = %q, want %q", rawURL, got, want)
		}
	}
}

func TestResolveURLRewritesBangumiAndVNDB(t *testing.T) {
	preference := hikarinagiPreference()

	cases := []struct {
		name    string
		source  enums.SourceType
		rawURL  string
		wantURL string
	}{
		{
			name:    "bangumi 走 bangumi 镜像",
			source:  enums.Bangumi,
			rawURL:  "https://lain.bgm.tv/pic/cover/l/c4/85/207413_frAAo.jpg",
			wantURL: HikarinagiBangumiImageProxyBaseURL + "https://lain.bgm.tv/pic/cover/l/c4/85/207413_frAAo.jpg",
		},
		{
			name:    "bangumi 镜像站同样按 bangumi 处理",
			source:  enums.BangumiMirror,
			rawURL:  "https://lain.bgm.tv/x.jpg",
			wantURL: HikarinagiBangumiImageProxyBaseURL + "https://lain.bgm.tv/x.jpg",
		},
		{
			name:    "vndb 走 vndb 镜像",
			source:  enums.VNDB,
			rawURL:  "https://t.vndb.org/cv.t/61/77161.jpg",
			wantURL: HikarinagiVNDBImageProxyBaseURL + "https://t.vndb.org/cv.t/61/77161.jpg",
		},
		{
			name:    "已经是指向镜像的地址不重复包裹",
			source:  enums.VNDB,
			rawURL:  HikarinagiVNDBImageProxyBaseURL + "https://t.vndb.org/cv.t/61/77161.jpg",
			wantURL: HikarinagiVNDBImageProxyBaseURL + "https://t.vndb.org/cv.t/61/77161.jpg",
		},
		{
			name:    "没有镜像的来源保持原样",
			source:  enums.Hikarinagi,
			rawURL:  "https://images.yurari.moe/images/x.webp",
			wantURL: "https://images.yurari.moe/images/x.webp",
		},
		{
			name:    "空地址返回空",
			source:  enums.VNDB,
			rawURL:  "  ",
			wantURL: "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ResolveURL(preference, testCase.source, testCase.rawURL); got != testCase.wantURL {
				t.Errorf("ResolveURL() = %q, want %q", got, testCase.wantURL)
			}
		})
	}
}

func TestResolveURLKeepsOriginalWhenPreferenceIsOriginal(t *testing.T) {
	rawURL := "https://t.vndb.org/cv.t/61/77161.jpg"
	if got := ResolveURL(OriginalPreference, enums.VNDB, rawURL); got != rawURL {
		t.Errorf("original 偏好不应改写地址，得到 %q", got)
	}
}

// 用户只把 Bangumi 设成 hikarinagi 时，VNDB 的地址必须保持原样。
func TestResolveURLHonoursPerSourcePreference(t *testing.T) {
	preference := Preference{
		Bangumi: enums.MetadataCoverSourceHikarinagi,
		VNDB:    enums.MetadataCoverSourceOriginal,
	}
	vndbURL := "https://t.vndb.org/cv.t/61/77161.jpg"
	if got := ResolveURL(preference, enums.VNDB, vndbURL); got != vndbURL {
		t.Errorf("VNDB 偏好为 original 时不应改写，得到 %q", got)
	}

	bangumiURL := "https://lain.bgm.tv/x.jpg"
	wantBangumi := HikarinagiBangumiImageProxyBaseURL + bangumiURL
	if got := ResolveURL(preference, enums.Bangumi, bangumiURL); got != wantBangumi {
		t.Errorf("Bangumi 偏好为 hikarinagi 时应改写，得到 %q", got)
	}
}

func TestResolveURLByHost(t *testing.T) {
	preference := hikarinagiPreference()
	rawURL := "https://lain.bgm.tv/pic/cover/l/c4/85/207413_frAAo.jpg"
	want := HikarinagiBangumiImageProxyBaseURL + rawURL
	if got := ResolveURLByHost(preference, rawURL); got != want {
		t.Errorf("ResolveURLByHost() = %q, want %q", got, want)
	}

	other := "https://cdn.ymgal.games/archive/main/09/0902b5.jpg"
	if got := ResolveURLByHost(preference, other); got != other {
		t.Errorf("识别不出来源时不应改写，得到 %q", got)
	}
}

func TestIsProxiedURL(t *testing.T) {
	if !IsProxiedURL(HikarinagiVNDBImageProxyBaseURL + "https://t.vndb.org/a.jpg") {
		t.Error("镜像地址应被识别")
	}
	if IsProxiedURL("https://t.vndb.org/a.jpg") {
		t.Error("原始地址不应被识别为镜像")
	}
}
