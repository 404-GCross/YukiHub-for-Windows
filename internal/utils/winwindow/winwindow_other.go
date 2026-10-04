//go:build !windows

package winwindow

import (
	"fmt"
	"unsafe"
)

// MakeNonActivating 在非 Windows 上不可用（本产品只做 Windows）。
func MakeNonActivating(unsafe.Pointer) error {
	return fmt.Errorf("仅 Windows 支持")
}

// SetRoundedCorners 在非 Windows 上不可用（本产品只做 Windows）。
func SetRoundedCorners(unsafe.Pointer, int) (string, error) {
	return "", fmt.Errorf("仅 Windows 支持")
}
