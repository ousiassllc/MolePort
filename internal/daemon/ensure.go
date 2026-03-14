package daemon

import (
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc/client"
)

const ensureRetryDelay = 200 * time.Millisecond

// startDaemonFunc はデーモン起動関数。テスト時に差し替え可能。
// NOTE: startDaemonFunc を差し替えるテストは t.Parallel() と併用不可。
var startDaemonFunc = StartDaemonProcess

// EnsureDaemon はデーモンが起動中であることを確認し、接続済みの IPCClient を返す。
// デーモンが起動していない場合は自動的にデーモンプロセスを起動してから接続する。
func EnsureDaemon(configDir string) (*client.IPCClient, error) {
	pidPath := PIDFilePath(configDir)
	running, _ := IsRunning(pidPath)
	if !running {
		if _, err := startDaemonFunc(configDir); err != nil {
			return nil, fmt.Errorf("failed to auto-start daemon: %w", err)
		}
	}

	c := client.NewIPCClient(SocketPath(configDir))
	if err := c.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	return c, nil
}

// EnsureDaemonWithRetry はデーモンが起動するまでリトライし、接続済みの IPCClient を返す。
func EnsureDaemonWithRetry(configDir string, maxWait time.Duration) (*client.IPCClient, error) {
	deadline := time.Now().Add(maxWait)
	for {
		c, err := EnsureDaemon(configDir)
		if err == nil {
			return c, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("daemon not ready after %s: %w", maxWait, err)
		}
		time.Sleep(ensureRetryDelay)
	}
}
