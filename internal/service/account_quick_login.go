package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yukihub/internal/applog"
	"yukihub/internal/common/vo"
	"yukihub/internal/service/yukihubaccount"
)

// 本文件实现 YukiHub 账号的第三方快捷登录（鲲站 / Hikarinagi）。
//
// 流程与手机版 AuthActivity + KungalOAuthCallbackActivity 完全一致：
//  1. 生成 PKCE verifier / state，起本地 loopback 回调服务，打开系统浏览器；
//  2. 用户在第三方授权后浏览器带着 code 回到 127.0.0.1；
//  3. 客户端把 code + verifier 交给自建后端（/auth/*/android_callback），
//     由后端向第三方换令牌并直接下发 YukiHub 会话——客户端接触不到第三方令牌，
//     密码授权等敏感逻辑全部收在服务端。
//
// 与 HikarinagiService/NextMoeService 的 StartAuth 不同：那两个是把第三方
// 令牌存进本库用于刮削；这里是「用第三方身份登录 YukiHub 账号」，落库的
// 是 YukiHub 会话（applySession）。

// 快捷登录渠道的桌面回调端口与路径（与其它 OAuth 服务一样走固定端口 loopback，
// RFC 8252 的 native app 标准做法）。
//
// **端口必须复用已登记过的回调**，不能再挑新的：
//   - 鲲站快捷登录与 NextMoe 刮削授权是**同一个 OAuth 应用**（client `16cc...`），
//     该应用在平台后台登记的 PC 回调是 `127.0.0.1:14792`（见 nextmoe_service.go，
//     也是 ADR-0003 说的「已登记、注册一次长期有效」），这里直接复用 14792；
//   - Hikarinagi 登录与 HikarinagiService 的授权共用 client `hkn_qtm...`，
//     已登记 `127.0.0.1:14791`，同样复用。
const (
	kungalCallbackPort          = 14792 // 与 NextMoeService 同应用同回调，不是 Bangumi 的 23679
	kungalCallbackPath          = "/callback"
	kungalRedirectURI           = "http://127.0.0.1:14792/callback"
	kungalAuthTimeout           = 10 * time.Minute
	hikarinagiLoginCallbackPort = 14791 // 与 HikarinagiService 的授权回调同端口同应用
	hikarinagiLoginRedirectURI  = "http://127.0.0.1:14791/callback"
)

// quickLoginOAuthSession 是一次快捷登录的本地回调会话。
type quickLoginOAuthSession struct {
	provider     yukihubaccount.QuickLoginProvider
	resultChan   chan quickLoginOAuthResult
	server       *http.Server
	listener     net.Listener
	state        string
	codeVerifier string
	redirectURI  string
}

type quickLoginOAuthResult struct {
	Code  string
	Error string
}

func newQuickLoginOAuthSession(provider yukihubaccount.QuickLoginProvider) (*quickLoginOAuthSession, error) {
	port := kungalCallbackPort
	redirectURI := kungalRedirectURI
	if provider == yukihubaccount.QuickLoginHikarinagi {
		port = hikarinagiLoginCallbackPort
		redirectURI = hikarinagiLoginRedirectURI
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("无法启动快捷登录本地回调服务（端口 %d 被占用？）: %w", port, err)
	}
	state, err := generateQuickLoginRandomValue(32)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("生成 OAuth state 失败: %w", err)
	}
	verifier, err := generateQuickLoginRandomValue(64)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("生成 PKCE verifier 失败: %w", err)
	}

	session := &quickLoginOAuthSession{
		provider:     provider,
		resultChan:   make(chan quickLoginOAuthResult, 1),
		listener:     listener,
		state:        state,
		codeVerifier: verifier,
		redirectURI:  redirectURI,
	}
	mux := http.NewServeMux()
	mux.HandleFunc(kungalCallbackPath, session.handleOAuthCallback)
	session.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := session.server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			session.trySendResult(quickLoginOAuthResult{Error: serveErr.Error()})
		}
	}()
	return session, nil
}

func (s *quickLoginOAuthSession) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	_ = s.listener.Close()
}

func (s *quickLoginOAuthSession) trySendResult(result quickLoginOAuthResult) {
	select {
	case s.resultChan <- result:
	default:
	}
}

func (s *quickLoginOAuthSession) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>登录失败</title></head><body><h1>登录失败</h1><p>请求方法无效</p><p>您可以关闭此窗口。</p></body></html>`)
		return
	}
	if !isLoopbackRequest(r.RemoteAddr) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>登录失败</title></head><body><h1>登录失败</h1><p>回调来源无效</p><p>请返回应用后重试。</p></body></html>`)
		return
	}
	query := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(s.state)) != 1 {
		s.trySendResult(quickLoginOAuthResult{Error: "登录状态校验失败，请重新尝试"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>登录失败</title></head><body><h1>登录失败</h1><p>登录状态校验失败</p><p>请返回应用后重试。</p></body></html>`)
		return
	}
	if oauthError := strings.TrimSpace(query.Get("error")); oauthError != "" {
		description := strings.TrimSpace(query.Get("error_description"))
		s.trySendResult(quickLoginOAuthResult{Error: strings.TrimSpace(oauthError + ": " + description)})
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>登录失败</title></head><body><h1>登录失败</h1><p>%s: %s</p><p>您可以关闭此窗口。</p></body></html>`, html.EscapeString(oauthError), html.EscapeString(description))
		return
	}
	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		s.trySendResult(quickLoginOAuthResult{Error: "未收到授权码"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>登录失败</title></head><body><h1>登录失败</h1><p>未收到授权码</p><p>您可以关闭此窗口。</p></body></html>`)
		return
	}
	s.trySendResult(quickLoginOAuthResult{Code: code})
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>登录成功</title></head><body><h1>登录成功！</h1><p>您可以关闭此窗口并返回应用。</p><script>window.close();</script></body></html>`)
}

// buildQuickLoginAuthorizeURL 构造授权页地址（授权码 + PKCE S256）。
func buildQuickLoginAuthorizeURL(provider yukihubaccount.QuickLoginProvider, session *quickLoginOAuthSession) string {
	challengeBytes := sha256.Sum256([]byte(session.codeVerifier))
	params := url.Values{
		"client_id":             {provider.ClientID()},
		"redirect_uri":          {session.redirectURI},
		"response_type":         {"code"},
		"state":                 {session.state},
		"scope":                 {provider.Scope()},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challengeBytes[:])},
		"code_challenge_method": {"S256"},
	}
	return provider.AuthorizeURL() + "?" + params.Encode()
}

func generateQuickLoginRandomValue(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// StartQuickLogin 用第三方快捷登录（provider: kungal / hikarinagi）登录 YukiHub 账号。
//
// 打开浏览器 → 等本地回调 → code 交给自建后端换 YukiHub 会话。
// 与手机版行为一致：成功后即进入登录态（含心跳），云同步开关保持原样。
func (s *AccountService) StartQuickLogin(provider string) (vo.AccountStatus, error) {
	p := yukihubaccount.QuickLoginProvider(strings.TrimSpace(provider))
	if p != yukihubaccount.QuickLoginKungal && p != yukihubaccount.QuickLoginHikarinagi {
		return vo.AccountStatus{}, fmt.Errorf("不支持的快捷登录渠道: %s", provider)
	}
	if strings.TrimSpace(p.AuthorizeURL()) == "" || strings.TrimSpace(p.ClientID()) == "" {
		return vo.AccountStatus{}, fmt.Errorf("快捷登录渠道未配置")
	}

	session, err := newQuickLoginOAuthSession(p)
	if err != nil {
		return vo.AccountStatus{}, err
	}
	defer session.shutdown()

	authURL := buildQuickLoginAuthorizeURL(p, session)
	if err := s.openURL(authURL); err != nil {
		return vo.AccountStatus{}, fmt.Errorf("打开快捷登录页面失败: %w", err)
	}

	timer := time.NewTimer(kungalAuthTimeout)
	defer timer.Stop()

	select {
	case result := <-session.resultChan:
		if result.Error != "" {
			return vo.AccountStatus{}, fmt.Errorf("快捷登录失败: %s", result.Error)
		}
		// 后端按 redirectUri 校验 code 的签发参数，必须原样回传本次会话的值。
		newSession, err := s.client.ExchangeQuickLoginCode(
			s.resolveContext(nil), p, result.Code, session.codeVerifier, session.redirectURI,
		)
		if err != nil {
			return vo.AccountStatus{}, err
		}
		if err := s.applySession(newSession); err != nil {
			return vo.AccountStatus{}, err
		}
		applog.LogInfof(s.ctx, "YukiHub 账号：快捷登录成功 provider=%s uid=%d", p, newSession.User.UID)
		return s.GetAccountStatus(), nil
	case <-timer.C:
		return vo.AccountStatus{}, fmt.Errorf("快捷登录超时，请重试")
	case <-s.resolveContext(s.ctx).Done():
		return vo.AccountStatus{}, s.resolveContext(s.ctx).Err()
	}
}
