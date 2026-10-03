//go:build !windows

package nativenotify

import "errors"

// ErrUnsupported 表示当前平台没有系统通知实现（YukiHub 目前只发布 Windows 版）。
var ErrUnsupported = errors.New("nativenotify: 仅支持 Windows")

// Notifier 在非 Windows 平台是空实现，调用方应当降级为只发应用内通知。
type Notifier struct{}

func New() (*Notifier, error) { return nil, ErrUnsupported }

func (n *Notifier) Notify(title, message string) error { return ErrUnsupported }

func (n *Notifier) Close() {}
