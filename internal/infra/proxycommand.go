package infra

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"sync"
	"time"
)

// proxyCommandAddr は ProxyCommand 経由接続用の net.Addr 実装。
type proxyCommandAddr struct {
	desc string
}

func (a proxyCommandAddr) Network() string { return "proxycommand" }
func (a proxyCommandAddr) String() string  { return a.desc }

// proxyCommandConn は ProxyCommand の stdin/stdout を net.Conn として扱うラッパー。
type proxyCommandConn struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	done      chan struct{}
	closeOnce sync.Once
}

func (c *proxyCommandConn) Read(b []byte) (int, error) {
	return c.stdout.Read(b)
}

func (c *proxyCommandConn) Write(b []byte) (int, error) {
	return c.stdin.Write(b)
}

func (c *proxyCommandConn) Close() error {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		_ = c.stdout.Close()

		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		// cmd.Wait() は goroutine 側で実行される。done チャネルで完了を待機する。
		<-c.done
	})
	return nil
}

func (c *proxyCommandConn) LocalAddr() net.Addr {
	return proxyCommandAddr{desc: "proxycommand-local"}
}

func (c *proxyCommandConn) RemoteAddr() net.Addr {
	return proxyCommandAddr{desc: c.cmd.String()}
}

// ProxyCommand 経由の場合、OS パイプには SetDeadline の概念がないため no-op とする。
// SSH ハンドシェイクのタイムアウト保護は機能しないが、ProxyCommand 自体の
// タイムアウト制御はコマンド側の責務とする（OpenSSH と同様の挙動）。
func (c *proxyCommandConn) SetDeadline(_ time.Time) error      { return nil }
func (c *proxyCommandConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *proxyCommandConn) SetWriteDeadline(_ time.Time) error { return nil }

// dialViaProxyCommand は ProxyCommand を起動し、その stdin/stdout を net.Conn として返す。
func dialViaProxyCommand(command string) (*proxyCommandConn, error) {
	cmd := exec.Command("sh", "-c", command) //nolint:gosec // ProxyCommand は SSH config 由来のユーザー設定値

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe for ProxyCommand %q: %w", command, err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe for ProxyCommand %q: %w", command, err)
	}

	// ProxyCommand の stderr はログに記録する。
	// os.Stderr に直接流すと TUI 表示を乱す可能性がある。
	cmd.Stderr = &proxyCommandStderrWriter{command: command}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ProxyCommand %q: %w", command, err)
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	return &proxyCommandConn{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		done:   done,
	}, nil
}

// proxyCommandStderrWriter は ProxyCommand の stderr 出力を slog 経由でログに記録する。
type proxyCommandStderrWriter struct {
	command string
}

func (w *proxyCommandStderrWriter) Write(p []byte) (int, error) {
	slog.Warn("ProxyCommand stderr", "command", w.command, "output", string(p))
	return len(p), nil
}
