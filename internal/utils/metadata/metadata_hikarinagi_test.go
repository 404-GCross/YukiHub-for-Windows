package metadata

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"yukihub/internal/common/enums"
	"yukihub/internal/version"
)

type hikarinagiRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn hikarinagiRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHikarinagiGetterUsesClientCredentialsAndCachesToken(t *testing.T) {
	previousClientID := version.HikarinagiMetadataClientID
	previousClientSecret := version.HikarinagiMetadataClientSecret
	previousLimiter := sharedMetadataRateLimiter
	t.Cleanup(func() {
		version.HikarinagiMetadataClientID = previousClientID
		version.HikarinagiMetadataClientSecret = previousClientSecret
		sharedMetadataRateLimiter = previousLimiter
		resetHikarinagiTokenCacheForTest()
	})
	version.HikarinagiMetadataClientID = "client-id"
	version.HikarinagiMetadataClientSecret = "client-secret"
	sharedMetadataRateLimiter = newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{})
	resetHikarinagiTokenCacheForTest()

	// 元数据应用凭据的 scope 必须与 Hikarinagi 授予该 client 的范围一致，
	// 写错会被服务端以 invalid_scope 直接拒绝（上游用的 catalog:full 不适用于本 client）。
	if hikarinagiScope != "catalog:read" {
		t.Fatalf("元数据应用凭据 scope = %q，应为 catalog:read", hikarinagiScope)
	}

	var tokenRequests int32
	client := &http.Client{Transport: hikarinagiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(body string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}

		switch {
		case req.URL.String() == hikarinagiTokenURL:
			atomic.AddInt32(&tokenRequests, 1)
			if req.Method != http.MethodPost {
				t.Fatalf("token method = %s", req.Method)
			}
			clientID, clientSecret, ok := req.BasicAuth()
			if !ok || clientID != "client-id" || clientSecret != "client-secret" {
				t.Fatalf("unexpected token basic auth: %q %q %v", clientID, clientSecret, ok)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if req.Form.Get("grant_type") != "client_credentials" || req.Form.Get("scope") != hikarinagiScope {
				t.Fatalf("unexpected token form: %#v", req.Form)
			}
			return response(`{"access_token":"test-access-token","token_type":"Bearer","expires_in":3600,"scope":"catalog:read"}`)
		case strings.Contains(req.URL.Path, "/v3/search"):
			assertHikarinagiBearerToken(t, req)
			if req.URL.Query().Get("q") != "CLANNAD" || req.URL.Query().Get("types") != "galgame" {
				t.Fatalf("unexpected search query: %s", req.URL.RawQuery)
			}
			return response(`{"success":true,"data":{"items":[{"type":"galgame","id":371,"title":"CLANNAD","subtitle":null,"developer":"Key","cover":null}],"meta":{"page":1,"page_size":1,"total_items":1,"item_count":1,"total_pages":1}},"request_id":"req-search"}`)
		case strings.HasSuffix(req.URL.Path, "/v3/galgames/371"):
			assertHikarinagiBearerToken(t, req)
			return response(`{"success":true,"data":{"id":371,"origin_title":"CLANNAD","trans_title":"克兰娜德","developer":"Key","covers":[{"url":"https://example.com/low.jpg","width":600,"height":800,"sexual":0,"violence":0,"votes":1},{"url":"https://example.com/best.jpg","width":600,"height":800,"sexual":0,"violence":0,"votes":9}],"release_date":"2004-04-28T00:00:00.000Z","origin_intro":"origin","trans_intro":"translated","nsfw":false,"tags":[{"name":"泣きゲー","likes":20},{"name":"学园","likes":10}],"rating":{"total":6,"score":9.1,"count":{"9":5,"10":1}}},"request_id":"req-detail"}`)
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	getter := NewHikarinagiInfoGetter(WithHTTPClient(client), WithTagLimit(2))
	result, err := getter.FetchMetadataByName("CLANNAD", "")
	if err != nil {
		t.Fatalf("FetchMetadataByName returned error: %v", err)
	}
	if result.Game.Name != "克兰娜德" || result.Game.Company != "Key" {
		t.Fatalf("unexpected game identity: %#v", result.Game)
	}
	if result.Game.CoverURL != "https://example.com/best.jpg" || result.Game.ReleaseDate != "2004-04-28" {
		t.Fatalf("unexpected cover/date: %#v", result.Game)
	}
	if result.Game.Rating != 9.1 {
		t.Fatalf("Rating = %v", result.Game.Rating)
	}
	if result.Game.SourceType != enums.Hikarinagi || result.Game.SourceID != "371" {
		t.Fatalf("unexpected source: %#v", result.Game)
	}
	if len(result.Tags) != 2 || result.Tags[0].Name != "泣きゲー" || result.Tags[1].Weight != 0.5 {
		t.Fatalf("unexpected tags: %#v", result.Tags)
	}
	if atomic.LoadInt32(&tokenRequests) != 1 {
		t.Fatalf("token requests = %d, want 1", tokenRequests)
	}
}

func TestHikarinagiGetterFetchMetadataUsesDetailDeveloper(t *testing.T) {
	previousLimiter := sharedMetadataRateLimiter
	t.Cleanup(func() {
		sharedMetadataRateLimiter = previousLimiter
	})
	sharedMetadataRateLimiter = newMetadataRateLimiter(map[MetadataSource]MetadataRateLimitPolicy{})

	client := &http.Client{Transport: hikarinagiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(body string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}

		switch {
		case strings.HasSuffix(req.URL.Path, "/v3/galgames/14792"):
			if got := req.Header.Get("Authorization"); got != "Bearer user-access-token" {
				t.Fatalf("Authorization = %q", got)
			}
			return response(`{"success":true,"data":{"id":14792,"origin_title":"デート・ア・ライブ 蓮ディストピア","trans_title":"约会大作战：莲反乌托邦","developer":" 株式会社コンパイルハート ","covers":[],"release_date":"2020-09-24T00:00:00.000Z","origin_intro":"intro","trans_intro":null,"nsfw":false,"tags":[],"rating":{"score":8.4}}}`)
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	result, err := NewHikarinagiInfoGetter(WithHTTPClient(client)).FetchMetadata("14792", "user-access-token")
	if err != nil {
		t.Fatalf("FetchMetadata returned error: %v", err)
	}
	if result.Game.Company != "株式会社コンパイルハート" {
		t.Fatalf("Company = %q", result.Game.Company)
	}
	if result.Game.Rating != 8.4 {
		t.Fatalf("Rating = %v", result.Game.Rating)
	}
}

// 元数据凭据是**内置**的（与 Android 端 metadata/HikarinagiClient.java 同源的应用级凭据），
// 开箱即可用，不再要求构建时注入。这里同时钉住两件事：
// 内置默认值存在（否则用户什么都没配就完全用不了 Hikarinagi 元数据），
// 以及注入值优先（便于轮换或自建替代应用）。
func TestHikarinagiMetadataCredentialsAreBuiltInAndInjectable(t *testing.T) {
	previousClientID := version.HikarinagiMetadataClientID
	previousClientSecret := version.HikarinagiMetadataClientSecret
	t.Cleanup(func() {
		version.HikarinagiMetadataClientID = previousClientID
		version.HikarinagiMetadataClientSecret = previousClientSecret
	})

	version.HikarinagiMetadataClientID = ""
	version.HikarinagiMetadataClientSecret = ""
	clientID, clientSecret := hikarinagiMetadataClientCredentials()
	if clientID == "" || clientSecret == "" {
		t.Fatalf("未注入时应回退到内置元数据凭据，实际得到 %q / %q", clientID, clientSecret)
	}
	if clientID != hikarinagiMetadataDefaultClientID {
		t.Fatalf("内置元数据 client id = %q, want %q", clientID, hikarinagiMetadataDefaultClientID)
	}

	version.HikarinagiMetadataClientID = "injected-id"
	version.HikarinagiMetadataClientSecret = "injected-secret"
	if clientID, clientSecret = hikarinagiMetadataClientCredentials(); clientID != "injected-id" || clientSecret != "injected-secret" {
		t.Fatalf("注入的元数据凭据未生效: %q / %q", clientID, clientSecret)
	}
}

func assertHikarinagiBearerToken(t *testing.T, req *http.Request) {
	t.Helper()
	if got := req.Header.Get("Authorization"); got != "Bearer test-access-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func resetHikarinagiTokenCacheForTest() {
	hikarinagiTokenCache.mu.Lock()
	defer hikarinagiTokenCache.mu.Unlock()
	hikarinagiTokenCache.clientID = ""
	hikarinagiTokenCache.clientSecret = ""
	hikarinagiTokenCache.token = ""
	hikarinagiTokenCache.expiresAt = time.Time{}
}
