package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yukihub/internal/appconf"
	"yukihub/internal/service/yukihubaccount"
)

// 契约文档：docs/yukihub-api-contract.md 第四节
//
// 这一组测试钉死两件容易做错的事：
//  1. 并发请求同时 401 时，refresh **只发一次**（否则刷新风暴 + 服务端限速）；
//  2. 网络异常 / 超时**不清登录态**（否则用户会莫名其妙被登出）。

// TestRefreshIsSharedAcrossConcurrent401 8 个并发请求同时 401，只应触发一次刷新。
func TestRefreshIsSharedAcrossConcurrent401(t *testing.T) {
	t.Parallel()

	service := NewAccountService()
	service.ctx = context.Background()
	service.config = &appconf.AppConfig{YukiHubAccountAccessToken: "access-1"}

	var refreshCalls int32
	release := make(chan struct{})
	service.refreshSessionFn = func() error {
		atomic.AddInt32(&refreshCalls, 1)
		// 卡住刷新，让其余并发请求都挤进「等待同一次刷新」的分支
		<-release
		return nil
	}

	const workers = 8
	var wg sync.WaitGroup
	results := make([]error, workers)
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			_, results[slot] = accountFetch(service, func(string) (string, error) {
				// 第一次带旧令牌必然 401；刷新后重试仍返回 401（服务端抖动场景）
				return "", yukihubaccount.ErrUnauthorized
			})
		}(index)
	}

	// 给并发请求一点时间汇合到等待分支
	time.Sleep(80 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&refreshCalls); got != 1 {
		t.Errorf("refresh 调用次数 = %d, want 1（并发 401 只应刷新一次）", got)
	}
	for index, err := range results {
		if !errors.Is(err, yukihubaccount.ErrUnauthorized) {
			t.Errorf("worker %d 的错误 = %v, want ErrUnauthorized", index, err)
		}
	}
}

// TestNetworkFailureKeepsSession 刷新时遇到网络异常，**保留**本地登录态。
func TestNetworkFailureKeepsSession(t *testing.T) {
	t.Parallel()

	service := NewAccountService()
	service.ctx = context.Background()
	config := &appconf.AppConfig{
		YukiHubAccountAccessToken:  "access-1",
		YukiHubAccountRefreshToken: "refresh-1",
	}
	service.config = config
	service.refreshSessionFn = func() error {
		return errors.New("请求账号服务失败: context deadline exceeded")
	}

	_, err := accountFetch(service, func(string) (string, error) {
		return "", yukihubaccount.ErrUnauthorized
	})
	if err == nil {
		t.Fatal("应当返回错误")
	}
	if config.YukiHubAccountAccessToken != "access-1" {
		t.Error("网络异常时不应清空 access token（会把用户直接登出）")
	}
	if config.YukiHubAccountRefreshToken != "refresh-1" {
		t.Error("网络异常时不应清空 refresh token")
	}
}

// TestSessionInvalidErrorClassification 只有「令牌失效 / 账号禁用」才允许清登录态。
func TestSessionInvalidErrorClassification(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"刷新返回 401", yukihubaccount.ErrUnauthorized, true},
		{"账号被封禁 403", yukihubaccount.ErrAccountDisabled, true},
		{"包装过的 401", errors.New("wrapped: " + yukihubaccount.ErrUnauthorized.Error()), false},
		{"网络超时", errors.New("context deadline exceeded"), false},
		{"服务端 500", errors.New("账号服务返回 HTTP 500"), false},
	}
	for _, tc := range cases {
		if got := isSessionInvalidError(tc.err); got != tc.want {
			t.Errorf("%s：isSessionInvalidError = %v, want %v", tc.name, got, tc.want)
		}
	}

	// 401 必须能被 errors.Is 识别（accountFetch 靠它触发刷新）
	if !isSessionInvalidError(yukihubaccount.ErrUnauthorized) {
		t.Error("ErrUnauthorized 应判定为需要重新登录")
	}
	if strings.Contains(yukihubaccount.ErrAccountDisabled.Error(), "HTTP") {
		t.Error("哨兵错误文案不该混入状态码")
	}
}
