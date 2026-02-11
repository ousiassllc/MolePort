package daemon

import (
	"fmt"

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
