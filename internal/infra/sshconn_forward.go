package infra

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// closeOnCancel は ctx がキャンセルされたときにリスナーを閉じる goroutine を起動する。
func closeOnCancel(ctx context.Context, listener net.Listener, label string, addr string) {
	go func() {
		<-ctx.Done()
		if err := listener.Close(); err != nil {
			slog.Debug("failed to close "+label+" listener", "addr", addr, "error", err)
		}
	}()
}

// LocalForward はローカルポートフォワーディング用のリスナーを作成する。
// このメソッドはリスナーの作成のみを行い、accept ループやデータ転送は行わない。
// 呼び出し元（ForwardManager）が返されたリスナーで accept ループを実行し、
// Dial() で取得した ssh.Client を使ってリモートへのデータブリッジを行う。
func (c *sshConnection) LocalForward(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	addr := net.JoinHostPort(core.LocalhostAddr, fmt.Sprintf("%d", localPort))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	closeOnCancel(ctx, listener, "local forward", addr)
	return listener, nil
}

// RemoteForward はリモートポートフォワーディング用のリスナーを作成する。
// このメソッドはリモート側リスナーの作成のみを行い、accept ループやデータ転送は行わない。
// 呼び出し元（ForwardManager）が返されたリスナーで accept ループを実行し、
// Dial() で取得した ssh.Client を使ってローカルへのデータブリッジを行う。
func (c *sshConnection) RemoteForward(ctx context.Context, remotePort int, localAddr string, remoteBindAddr string) (net.Listener, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	if remoteBindAddr == "" {
		remoteBindAddr = core.LocalhostAddr
	}
	addr := net.JoinHostPort(remoteBindAddr, fmt.Sprintf("%d", remotePort))
	listener, err := client.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen remotely on %s: %w", addr, err)
	}

	closeOnCancel(ctx, listener, "remote forward", addr)
	return listener, nil
}

// DynamicForward はダイナミックフォワーディング（SOCKS プロキシ）用のリスナーを作成する。
// このメソッドはリスナーの作成のみを行い、SOCKS プロトコル処理やデータ転送は行わない。
// 呼び出し元（ForwardManager）が返されたリスナーで accept ループを実行し、
// Dial() で取得した ssh.Client を使って SOCKS プロキシのデータブリッジを行う。
func (c *sshConnection) DynamicForward(ctx context.Context, localPort int) (net.Listener, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	addr := net.JoinHostPort(core.LocalhostAddr, fmt.Sprintf("%d", localPort))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	closeOnCancel(ctx, listener, "dynamic forward", addr)
	return listener, nil
}

// defaultAliveTimeout は IsAlive が keepalive の応答を待つ上限。
const defaultAliveTimeout = 15 * time.Second

// IsAlive は keepalive リクエストで接続の生存を確認する。
// 応答が defaultAliveTimeout 以内に返らない場合は死んでいると判定する。
func (c *sshConnection) IsAlive() bool {
	return c.checkAlive(context.Background(), defaultAliveTimeout)
}

// checkAlive は keepalive リクエストを送り、timeout 以内に成功応答が返れば true を返す。
// crypto/ssh の SendRequest にはタイムアウトがないため、別 goroutine で実行して待ち時間を制限する。
// 応答がなく無音で切れた接続は、タイムアウト時に Close して送信 goroutine を解放する。
// ctx がキャンセルされた場合は false を返すが、接続は閉じない（呼び出し元が Disconnect で閉じる）。
func (c *sshConnection) checkAlive(ctx context.Context, timeout time.Duration) bool {
	client := c.getClient()
	if client == nil {
		return false
	}

	result := make(chan error, 1)
	go func() {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		result <- err
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-result:
		return err == nil
	case <-ctx.Done():
		return false
	case <-timer.C:
		slog.Warn("keepalive response timed out, closing connection", "timeout", timeout)
		_ = c.Close()
		return false
	}
}

// KeepAlive は定期的に keepalive リクエストを送信する。
// 応答は interval 以内に返る必要があり、返らない場合は接続断とみなして戻る。
func (c *sshConnection) KeepAlive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.checkAlive(ctx, interval) {
				if ctx.Err() == nil {
					slog.Warn("keepalive failed, connection may be lost")
				}
				return
			}
		}
	}
}
