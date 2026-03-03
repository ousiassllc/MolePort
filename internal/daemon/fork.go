package daemon

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// IsDaemonMode は os.Args に --daemon-mode フラグが含まれているかを返す。
func IsDaemonMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--daemon-mode" {
			return true
		}
	}
	return false
}

// StartDaemonProcess は現在のバイナリをデーモンプロセスとしてフォークする。
// 起動したプロセスの PID を返す。
func StartDaemonProcess(configDir string) (int, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("get executable: %w", err)
	}

	args := []string{executable, "--daemon-mode", "--config-dir", configDir}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return 0, fmt.Errorf("open devnull: %w", err)
	}
	defer devNull.Close()

	attr := &os.ProcAttr{
		Dir:   "/",
		Env:   os.Environ(),
		Files: []*os.File{devNull, devNull, devNull},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	}

	proc, err := os.StartProcess(executable, args, attr)
	if err != nil {
		return 0, fmt.Errorf("start process: %w", err)
	}

	pid := proc.Pid
	proc.Release()

	// デーモンの起動完了を待機（ソケット接続を試行）
	socketPath := SocketPath(configDir)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return pid, nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return pid, nil // ソケット未準備でも PID を返す
}
