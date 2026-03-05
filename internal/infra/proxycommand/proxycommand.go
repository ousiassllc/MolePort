package proxycommand

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// addr は ProxyCommand 経由接続用の net.Addr 実装。
type addr struct {
	desc string
}

func (a addr) Network() string { return "proxycommand" }
func (a addr) String() string  { return a.desc }

// conn は ProxyCommand の stdin/stdout を net.Conn として扱うラッパー。
type conn struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	done      chan struct{}
	closeOnce sync.Once
}

func (c *conn) Read(b []byte) (int, error) {
	return c.stdout.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	return c.stdin.Write(b)
}

func (c *conn) Close() error {
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

func (c *conn) LocalAddr() net.Addr {
	return addr{desc: "proxycommand-local"}
}

func (c *conn) RemoteAddr() net.Addr {
	return addr{desc: c.cmd.String()}
}

// ProxyCommand 経由の場合、OS パイプには SetDeadline の概念がないため no-op とする。
// SSH ハンドシェイクのタイムアウト保護は機能しないが、ProxyCommand 自体の
// タイムアウト制御はコマンド側の責務とする（OpenSSH と同様の挙動）。
func (c *conn) SetDeadline(_ time.Time) error      { return nil }
func (c *conn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *conn) SetWriteDeadline(_ time.Time) error { return nil }

// Dial は ProxyCommand を起動し、その stdin/stdout を net.Conn として返す。
func Dial(command string) (net.Conn, error) {
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
	cmd.Stderr = &stderrWriter{command: command}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ProxyCommand %q: %w", command, err)
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	return &conn{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		done:   done,
	}, nil
}

// stderrWriter は ProxyCommand の stderr 出力を slog 経由でログに記録する。
type stderrWriter struct {
	command string
}

func (w *stderrWriter) Write(p []byte) (int, error) {
	slog.Warn("ProxyCommand stderr", "command", commandName(w.command), "output", string(p))
	return len(p), nil
}

// commandName はコマンド文字列から最初のトークン（実行ファイル名）のみを返す。
// ログ出力時に引数（ホスト名やポート等）をマスクする目的で使用する。
func commandName(command string) string {
	command = strings.TrimSpace(command)
	if name, _, ok := strings.Cut(command, " "); ok {
		return name
	}
	return command
}

// ExpandCommand は ProxyCommand 文字列内の SSH トークンを展開する。
// サポートするトークン:
//
//	%h → リモートホスト名
//	%p → ポート番号
//	%r → リモートユーザー名
//	%% → リテラルの %
//
// 上記以外のトークン（%n, %d 等）は未展開のまま保持される。
// ProxyCommand が設定されている場合は ProxyJump より優先される（OpenSSH の挙動に準拠）。
func ExpandCommand(command, host string, port int, user string) string {
	if command == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(command))

	for i := 0; i < len(command); i++ {
		if command[i] == '%' && i+1 < len(command) {
			switch command[i+1] {
			case 'h':
				b.WriteString(host)
				i++
			case 'p':
				b.WriteString(strconv.Itoa(port))
				i++
			case 'r':
				b.WriteString(user)
				i++
			case '%':
				b.WriteByte('%')
				i++
			default:
				b.WriteByte(command[i])
			}
		} else {
			b.WriteByte(command[i])
		}
	}
	return b.String()
}
