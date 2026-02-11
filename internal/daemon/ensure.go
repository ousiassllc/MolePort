package daemon

import (
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// EnsureDaemon はデーモンが起動中であることを確認し、接続済みの IPCClient を返す。
// デーモンが起動していない場合はエラーを返す。
func EnsureDaemon(configDir string) (*ipc.IPCClient, error) {
	pidPath := PIDFilePath(configDir)
	running, _ := IsRunning(pidPath)
	if !running {
		return nil, fmt.Errorf("daemon is not running; start it with: moleport daemon start")
	}

	client := ipc.NewIPCClient(SocketPath(configDir))
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	return client, nil
}

// EnsureDaemonWithRetry はデーモンが起動するまでリトライし、接続済みの IPCClient を返す。
func EnsureDaemonWithRetry(configDir string, maxWait time.Duration) (*ipc.IPCClient, error) {
	deadline := time.Now().Add(maxWait)
	for {
		client, err := EnsureDaemon(configDir)
		if err == nil {
			return client, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("daemon not ready after %s: %w", maxWait, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
