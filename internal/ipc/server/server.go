// Package ipcserver 在 GUI 进程里开一个本地 HTTP 端点，供 yukihub:// 协议转发使用。
//
// 场景：用户点击 yukihub://install?... / yukihub://launch?... 时如果已经有一个
// YukiHub 在跑，新起的进程会把请求 POST 到这里（见 internal/ipc/core），
// 由正在运行的实例处理，而不是再开一份。
//
// 历史：这个端点原本还承载 yukihubcli 的 /run（把命令行参数转发给 GUI 执行）。
// CLI 已下线，端点只剩协议转发需要的 /ping、/install、/launch。
package ipcserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"yukihub/internal/common/vo"

	"yukihub/internal/applog"
	"yukihub/internal/service"
	"yukihub/internal/wailsruntime"
)

// StartServer 启动协议转发端点 (在 GUI 进程中运行)
func StartServer(ctx context.Context, startService *service.StartService, runtime wailsruntime.Runtime) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	// /install: 接收来自新启动实例转发的 yukihub:// 安装请求
	mux.HandleFunc("/install", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req vo.InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		// 不直接开始下载，只推送事件让前端弹出确认窗口
		// 用户确认后前端调用 DownloadService.StartDownload
		runtime.Emit("install:pending", req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(InstallResponse{TaskID: ""})
	})

	// /launch: 接收来自新启动实例转发的 yukihub:// 启动请求
	mux.HandleFunc("/launch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req vo.ProtocolLaunchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		resp := LaunchResponse{}
		if err := startService.HandleProtocolLaunch(req); err != nil {
			resp.Error = err.Error()
		} else {
			resp.Started = true
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	listener, port, err := chooseIPCListener()
	if err != nil {
		applog.LogErrorf(ctx, "IPC Server failed to acquire port: %v", err)
		return nil
	}
	savePort(port)
	server := &http.Server{Handler: mux}

	applog.LogInfof(ctx, "IPC Server starting on %s", listener.Addr().String())
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			applog.LogErrorf(ctx, "IPC Server failed: %v", err)
		}
	}()

	return server
}

// ShutdownServer 关闭 IPC 服务器并清理 endpoint 文件
func ShutdownServer(server *http.Server) error {
	if server == nil {
		clearSavedPort()
		return nil
	}

	defer clearSavedPort()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
