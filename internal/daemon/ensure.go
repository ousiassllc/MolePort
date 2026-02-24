package daemon

import (
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc/client"
)

// EnsureDaemon はデーモンが起動中であることを確認し、接続済みの IPCClient を返す。
// デーモンが起動していない場合はエラーを返す。
func EnsureDaemon(configDir string) (*client.IPCClient, error) {
	pidPath := PIDFilePath(configDir)
	running, _ := IsRunning(pidPath)
	if !running {
		return nil, fmt.Errorf("daemon is not running; start it with: moleport daemon start")
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
		time.Sleep(200 * time.Millisecond)
	}
}
