package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/daemon/pidfile"
	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

const (
	// TakeoverTimeout は引き継ぎ時に PID ロックの解放を待つ標準の最大時間。
	TakeoverTimeout = 10 * time.Second

	takeoverPollInterval = 100 * time.Millisecond
	takeoverCallTimeout  = 5 * time.Second
)

var (
	// ErrTakeoverUnresponsive は PID ロックが保持されているのに IPC ソケットが応答しないことを表す。
	// 応答しない旧デーモンを強制終了しないため、呼び出し元がユーザーに対処（daemon kill）を案内する。
	ErrTakeoverUnresponsive = errors.New("daemon holds the pid lock but does not respond on the socket")

	// ErrTakeoverTimeout は daemon.shutdown を送信したが、時間内に PID ロックが解放されなかったことを表す。
	ErrTakeoverTimeout = errors.New("daemon did not release the pid lock in time")
)

// Takeover は稼働中の headless デーモンを daemon.shutdown (purge=false) で停止し、
// PID ファイルのロックが解放されるまで最大 timeout 待機する。
//
// 戻り値:
//   - ロックが保持されていない（PID ファイルなし / stale PID）場合は (0, nil)。引き継ぎ不要。
//   - 引き継ぎに成功した場合は旧デーモンの PID と nil。
//   - ロック保持中だがソケットが応答しない場合は ErrTakeoverUnresponsive（強制終了しない）。
//   - timeout 以内にロックが解放されない場合は ErrTakeoverTimeout。
//
// エラー時も判明している旧デーモンの PID を返す。
func Takeover(configDir string, timeout time.Duration) (prevPID int, err error) {
	pidPath := PIDFilePath(configDir)
	locked, pid := pidfile.IsLocked(pidPath)
	if !locked {
		return 0, nil
	}

	if err := requestShutdown(SocketPath(configDir)); err != nil {
		// 応答直前にデーモンが終了して接続が切れた場合は引き継ぎ成功として扱う。
		if stillLocked, _ := pidfile.IsLocked(pidPath); !stillLocked {
			return pid, nil
		}
		return pid, fmt.Errorf("%w: %w", ErrTakeoverUnresponsive, err)
	}

	deadline := time.Now().Add(timeout)
	for {
		if stillLocked, _ := pidfile.IsLocked(pidPath); !stillLocked {
			return pid, nil
		}
		if !time.Now().Before(deadline) {
			return pid, fmt.Errorf("%w (waited %s)", ErrTakeoverTimeout, timeout)
		}
		time.Sleep(takeoverPollInterval)
	}
}

// requestShutdown は socketPath のデーモンに purge=false の daemon.shutdown を送信する。
func requestShutdown(socketPath string) error {
	c := client.NewIPCClient(socketPath)
	if err := c.Connect(); err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), takeoverCallTimeout)
	defer cancel()

	var result protocol.DaemonShutdownResult
	if err := c.Call(ctx, "daemon.shutdown", protocol.DaemonShutdownParams{Purge: false}, &result); err != nil {
		return fmt.Errorf("send daemon.shutdown: %w", err)
	}
	return nil
}
