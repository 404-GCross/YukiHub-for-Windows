// Package ipccore 是 yukihub:// 协议转发的客户端侧。
//
// 用途：用户点击 yukihub:// 链接时可能已经有一个 YukiHub 在跑，这时新起的
// 进程必须把请求交给那个进程（IPC 通道），而不是自己再开一份。
// 服务端在 internal/ipc/server。
//
// 历史：这里原本还带着一整套 yukihubcli 命令行工具（/run 端点、命令执行、
// cobra 命令树）。CLI 已下线，只保留协议转发真正需要的那部分。
package ipccore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	ServerAddr  = "127.0.0.1"
	Port        = 56789
	PingTimeout = 500 * time.Millisecond
)

type endpointInfo struct {
	Port int `json:"port"`
}

func serverURLForPort(port int) string {
	return fmt.Sprintf("http://%s:%d", ServerAddr, port)
}

func endpointFilePath() string {
	return filepath.Join(os.TempDir(), "yukihub_ipc_endpoint.json")
}

func readSavedPort() (int, bool) {
	content, err := os.ReadFile(endpointFilePath())
	if err != nil {
		return 0, false
	}

	var info endpointInfo
	if err := json.Unmarshal(content, &info); err != nil {
		return 0, false
	}

	if info.Port <= 0 {
		return 0, false
	}

	return info.Port, true
}

func candidateServerURLs() []string {
	urls := make([]string, 0, 2)
	seen := make(map[string]struct{})

	add := func(u string) {
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}

	if savedPort, ok := readSavedPort(); ok {
		add(serverURLForPort(savedPort))
	}
	add(serverURLForPort(Port))

	return urls
}

func pingServer(url string) bool {
	client := http.Client{Timeout: PingTimeout}
	resp, err := client.Get(url + "/ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func findRunningServerURL() (string, bool) {
	for _, url := range candidateServerURLs() {
		if pingServer(url) {
			return url, true
		}
	}
	return "", false
}

func IsServerRunning() bool {
	_, ok := findRunningServerURL()
	return ok
}

func RemoteInstall(req interface{}) error {
	serverURL, ok := findRunningServerURL()
	if !ok {
		return fmt.Errorf("failed to connect to YukiHub: IPC server not running")
	}

	jsonBody, _ := json.Marshal(req)
	resp, err := http.Post(serverURL+"/install", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to connect to YukiHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("YukiHub returned error status: %d", resp.StatusCode)
	}

	return nil
}

func RemoteLaunch(req interface{}) error {
	serverURL, ok := findRunningServerURL()
	if !ok {
		return fmt.Errorf("failed to connect to YukiHub: IPC server not running")
	}

	jsonBody, _ := json.Marshal(req)
	resp, err := http.Post(serverURL+"/launch", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to connect to YukiHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("YukiHub returned error status: %d", resp.StatusCode)
	}

	var launchResp LaunchResponse
	if err := json.NewDecoder(resp.Body).Decode(&launchResp); err != nil {
		return fmt.Errorf("failed to decode launch response: %w", err)
	}
	if launchResp.Error != "" {
		return fmt.Errorf("%s", launchResp.Error)
	}

	return nil
}

type LaunchResponse struct {
	Started bool   `json:"started"`
	Error   string `json:"error,omitempty"`
}
