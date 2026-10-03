package yukihubaccount

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// 契约文档：docs/yukihub-api-contract.md
//
// 这一组测试的作用是**钉死"敏感字段绝不进 URL"**。旧实现把邮箱 / 密码 /
// 验证码拼在 query string 里，会被服务器访问日志、代理日志、Referer 原样记录；
// 任何人把 POST 改回 GET，这些测试会立刻失败。

const okSessionBody = `{"accessToken":"access-1","refreshToken":"refresh-1",` +
	`"user":{"id":"7","uid":7,"nickname":"Yuki","email":"a@example.com"}}`

// recordedRequest 记录服务端实际收到的请求。
type recordedRequest struct {
	Method      string
	Path        string
	RawQuery    string
	RequestURI  string
	ContentType string
	Body        string
}

// newRecordingServer 起一个记录请求的测试服务器，handler 由调用方给。
func newRecordingServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]recordedRequest, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	requests := make([]recordedRequest, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, recordedRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			RawQuery:    r.URL.RawQuery,
			RequestURI:  r.RequestURI,
			ContentType: r.Header.Get("Content-Type"),
			Body:        string(raw),
		})
		mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &requests, &mu
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// assertNoSecretInURL 断言敏感值既不在 query 也不在完整请求行里。
func assertNoSecretInURL(t *testing.T, request recordedRequest, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		if strings.Contains(request.RequestURI, secret) {
			t.Errorf("敏感值 %q 出现在请求行里：%s", secret, request.RequestURI)
		}
		if strings.Contains(request.RawQuery, secret) {
			t.Errorf("敏感值 %q 出现在 query 里：%s", secret, request.RawQuery)
		}
	}
}

func assertJSONRequest(t *testing.T, request recordedRequest, wantMethod, wantPath string) {
	t.Helper()
	if request.Method != wantMethod {
		t.Errorf("method = %s, want %s", request.Method, wantMethod)
	}
	if request.Path != wantPath {
		t.Errorf("path = %s, want %s", request.Path, wantPath)
	}
	if !strings.Contains(request.ContentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", request.ContentType)
	}
	if request.RawQuery != "" {
		t.Errorf("请求不该带 query，实际 = %q", request.RawQuery)
	}
}

// TestLoginUsesPostWithJSONBody 登录必须 POST + JSON，密码不得进 URL。
func TestLoginUsesPostWithJSONBody(t *testing.T) {
	t.Parallel()

	const password = "super-secret-pw"
	server, requests, mu := newRecordingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, okSessionBody)
	})

	client := NewClient(server.URL)
	session, err := client.Login(context.Background(), "a@example.com", password)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if session.AccessToken != "access-1" || session.RefreshToken != "refresh-1" {
		t.Fatalf("会话解析异常：%#v", session)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*requests) != 1 {
		t.Fatalf("请求次数 = %d, want 1", len(*requests))
	}
	request := (*requests)[0]
	assertJSONRequest(t, request, http.MethodPost, "/auth/login")
	assertNoSecretInURL(t, request, password, "a@example.com")
	if !strings.Contains(request.Body, `"password":"`+password+`"`) {
		t.Errorf("请求体里应带 password，实际 body = %s", request.Body)
	}
	if !strings.Contains(request.Body, `"email":"a@example.com"`) {
		t.Errorf("请求体里应带 email，实际 body = %s", request.Body)
	}
}

// TestRegisterUsesPostWithJSONBody 注册的密码与验证码都不得进 URL。
func TestRegisterUsesPostWithJSONBody(t *testing.T) {
	t.Parallel()

	const (
		password = "reg-secret-pw"
		code     = "123456"
	)
	server, requests, mu := newRecordingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		// 契约：注册成功是 201
		writeJSON(w, http.StatusCreated, okSessionBody)
	})

	client := NewClient(server.URL)
	if _, err := client.Register(context.Background(), "a@example.com", password, "Yuki", code); err != nil {
		t.Fatalf("Register: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	request := (*requests)[0]
	assertJSONRequest(t, request, http.MethodPost, "/auth/register")
	assertNoSecretInURL(t, request, password, code)
	if !strings.Contains(request.Body, `"code":"`+code+`"`) {
		t.Errorf("请求体里应带 code，实际 body = %s", request.Body)
	}
}

// TestSendCodeUsesPostWithJSONBody 注册码与重置码都走 POST。
func TestSendCodeUsesPostWithJSONBody(t *testing.T) {
	t.Parallel()

	cases := []struct {
		purpose string
		want    string
	}{
		{CodePurposeRegister, "/auth/send_code"},
		{CodePurposeReset, "/auth/send_reset_code"},
	}
	for _, tc := range cases {
		t.Run(tc.purpose, func(t *testing.T) {
			t.Parallel()
			server, requests, mu := newRecordingServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, `{"success":true,"message":"验证码已发送至邮箱"}`)
			})

			client := NewClient(server.URL)
			if err := client.SendCode(context.Background(), "a@example.com", tc.purpose); err != nil {
				t.Fatalf("SendCode: %v", err)
			}

			mu.Lock()
			defer mu.Unlock()
			request := (*requests)[0]
			assertJSONRequest(t, request, http.MethodPost, tc.want)
			assertNoSecretInURL(t, request, "a@example.com")
		})
	}
}

// TestRefreshAndResetPasswordStayOnPost 防止有人把这两个接口改回 GET。
func TestRefreshAndResetPasswordStayOnPost(t *testing.T) {
	t.Parallel()

	server, requests, mu := newRecordingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, okSessionBody)
	})
	client := NewClient(server.URL)

	if _, err := client.Refresh(context.Background(), "refresh-1"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if err := client.ResetPassword(context.Background(), "a@example.com", "123456", "new-pw"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*requests) != 2 {
		t.Fatalf("请求次数 = %d, want 2", len(*requests))
	}
	refresh := (*requests)[0]
	assertJSONRequest(t, refresh, http.MethodPost, "/auth/refresh")
	assertNoSecretInURL(t, refresh, "refresh-1")
	reset := (*requests)[1]
	assertJSONRequest(t, reset, http.MethodPost, "/auth/reset_password")
	assertNoSecretInURL(t, reset, "123456", "new-pw")
}

// TestHTMLResponseIsNotParsed 拿到 HTML 说明打到了非 API 地址或服务器异常，
// 必须直接报「服务异常」，不能解析、更不能报成「响应缺少字段」。
func TestHTMLResponseIsNotParsed(t *testing.T) {
	t.Parallel()

	server, _, _ := newRecordingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<!DOCTYPE html><html><body>502 Bad Gateway</body></html>")
	})

	client := NewClient(server.URL)
	_, err := client.Login(context.Background(), "a@example.com", "pw")
	if !errors.Is(err, ErrServiceAbnormal) {
		t.Fatalf("err = %v, want ErrServiceAbnormal", err)
	}
	if strings.Contains(err.Error(), "缺少访问令牌") {
		t.Errorf("不该把 HTML 当成缺字段的会话响应：%v", err)
	}
}

// TestBusinessErrorMessagesArePreserved 服务端 error 文案原样透出，
// 且 400/401/403/429 各自语义正确。
func TestBusinessErrorMessagesArePreserved(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		status     int
		body       string
		wantErr    string
		wantMatch  func(error) bool
		wantAbsent string
	}{
		{
			name:   "缺参数 400",
			status: http.StatusBadRequest,
			body:   `{"error":"请填写邮箱和密码"}`,
			// 400 是业务错误：既不是令牌失效也不是账号禁用
			wantMatch: func(err error) bool {
				return !errors.Is(err, ErrUnauthorized) && !errors.Is(err, ErrAccountDisabled)
			},
		},
		{
			name:      "密码错 401",
			status:    http.StatusUnauthorized,
			body:      `{"error":"密码错误"}`,
			wantMatch: func(err error) bool { return errors.Is(err, ErrUnauthorized) },
		},
		{
			name:      "封禁 403",
			status:    http.StatusForbidden,
			body:      `{"error":"账号已被封禁：刷屏"}`,
			wantMatch: func(err error) bool { return errors.Is(err, ErrAccountDisabled) },
		},
		{
			name:      "限速 429",
			status:    http.StatusTooManyRequests,
			body:      `{"error":"登录尝试次数过多，请 600 秒后再试"}`,
			wantMatch: func(err error) bool { return !errors.Is(err, ErrUnauthorized) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, requests, mu := newRecordingServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, tc.status, tc.body)
			})
			client := NewClient(server.URL)

			_, err := client.Login(context.Background(), "a@example.com", "pw")
			if err == nil {
				t.Fatal("应当返回错误")
			}
			if !tc.wantMatch(err) {
				t.Errorf("错误语义不符：%v", err)
			}
			// 文案必须**原样**透出：不能只包含、还多出客户端前缀
			// （之前登录 401 会变成「登录状态已失效: 密码错误」）
			message := extractServerMessage(tc.body)
			if message != "" && err.Error() != message {
				t.Errorf("服务端文案 %q 未原样透出，实际 = %q", message, err.Error())
			}
			// 429 绝不能自动重试
			mu.Lock()
			count := len(*requests)
			mu.Unlock()
			if count != 1 {
				t.Errorf("请求次数 = %d, want 1（错误不得自动重试）", count)
			}
		})
	}
}

// extractServerMessage 从响应体里取出 error 文案（测试辅助）。
func extractServerMessage(body string) string {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return ""
	}
	for _, key := range []string{"error", "message", "msg"} {
		if value, ok := decoded[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}
