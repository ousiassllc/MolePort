package daemon

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// IsDaemonMode checks if --daemon-mode flag is present in os.Args.
func IsDaemonMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--daemon-mode" {
			return true
		}
	}
	return false
}

// StartDaemonProcess forks the current binary as a daemon process.
// Returns the PID of the spawned process.
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

	// Wait for daemon to be ready (poll socket)
	socketPath := SocketPath(configDir)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			return pid, nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return pid, nil // Return PID even if socket not ready yet
}
