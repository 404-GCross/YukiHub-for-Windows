package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"yukihub/internal/protocol"
	"yukihub/internal/utils/apputils"
)

// PortableSetupService exposes setup helpers needed by portable and AppImage
// builds. Packaged installer builds receive their custom URL protocol
// association from Wails.
type PortableSetupService struct {
	ctx context.Context
}

func NewPortableSetupService() *PortableSetupService {
	return &PortableSetupService{}
}

//wails:ignore
func (s *PortableSetupService) Init(ctx context.Context) {
	s.ctx = ctx
}

// PortableProtocolStatus describes the current yukihub:// scheme binding.
type PortableProtocolStatus struct {
	Registered     bool   `json:"registered"`
	RegisteredPath string `json:"registeredPath"`
	CurrentPath    string `json:"currentPath"`
	UpToDate       bool   `json:"upToDate"`
}

// PortableSetupStatus is the aggregate snapshot consumed by the settings UI.
type PortableSetupStatus struct {
	BuildMode      string                 `json:"buildMode"`
	IsPortable     bool                   `json:"isPortable"`
	Platform       string                 `json:"platform"`
	ExecutablePath string                 `json:"executablePath"`
	Protocol       PortableProtocolStatus `json:"protocol"`
}

// GetStatus returns the portable yukihub:// protocol registration state.
func (s *PortableSetupService) GetStatus() (PortableSetupStatus, error) {
	status := PortableSetupStatus{
		BuildMode:  apputils.GetBuildMode(),
		IsPortable: apputils.IsPortableMode(),
		Platform:   runtime.GOOS,
	}

	if exe, err := apputils.GetLaunchExecutablePath(); err == nil {
		if abs, absErr := filepath.Abs(exe); absErr == nil {
			status.ExecutablePath = abs
		} else {
			status.ExecutablePath = exe
		}
	} else if exe, execErr := os.Executable(); execErr == nil {
		if abs, absErr := filepath.Abs(exe); absErr == nil {
			status.ExecutablePath = abs
		} else {
			status.ExecutablePath = exe
		}
	}

	status.Protocol.CurrentPath = status.ExecutablePath
	registeredExe, err := protocol.GetRegisteredURLSchemeExe()
	if err != nil {
		return status, fmt.Errorf("query portable protocol status: %w", err)
	}
	status.Protocol.RegisteredPath = registeredExe
	status.Protocol.Registered = registeredExe != ""
	status.Protocol.UpToDate = status.Protocol.Registered && protocolRegistrationMatchesPath(registeredExe, status.ExecutablePath)

	return status, nil
}

func protocolRegistrationMatchesPath(registeredExe string, executablePath string) bool {
	if strings.TrimSpace(executablePath) == "" {
		return false
	}
	return protocol.HandlerMatchesTarget(registeredExe, executablePath)
}

// RegisterProtocol writes the yukihub:// association required by local builds.
// Installed builds are managed by Wails during packaging.
func (s *PortableSetupService) RegisterProtocol() (PortableSetupStatus, error) {
	if !supportsLocalIntegrationSetup() {
		return PortableSetupStatus{}, fmt.Errorf("安装版协议由 Wails 安装程序管理")
	}
	exePath, err := apputils.GetLaunchExecutablePath()
	if err != nil {
		return PortableSetupStatus{}, fmt.Errorf("resolve local executable: %w", err)
	}
	if err := protocol.RegisterPortableURLScheme(exePath); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("register local protocol: %w", err)
	}
	return s.GetStatus()
}

// UnregisterProtocol removes the current-user association created for a local
// build.
func (s *PortableSetupService) UnregisterProtocol() (PortableSetupStatus, error) {
	if !supportsLocalIntegrationSetup() {
		return PortableSetupStatus{}, fmt.Errorf("安装版协议由 Wails 安装程序管理")
	}
	if err := protocol.UnregisterPortableURLScheme(); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("unregister portable protocol: %w", err)
	}
	return s.GetStatus()
}

func supportsLocalIntegrationSetup() bool {
	return apputils.IsPortableMode() || apputils.IsAppImageMode()
}
