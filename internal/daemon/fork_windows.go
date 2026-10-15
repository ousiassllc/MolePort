//go:build windows

package daemon

import (
	"errors"
	"fmt"
)

// StartDaemonProcess は Windows では非対応。
// Windows の GUI はデーモンを fork せず、同一プロセスに内蔵して起動する。
func StartDaemonProcess(_ string) (int, error) {
	return 0, fmt.Errorf("fork daemon on windows: %w", errors.ErrUnsupported)
}
