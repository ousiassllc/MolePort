package infra

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestNewSSHConnection_IsAliveReturnsFalse(t *testing.T) {
	conn := NewSSHConnection()
	if conn.IsAlive() {
		t.Error("IsAlive should return false when not connected")
	}
}

func TestSSHConnection_CloseNilIsSafe(t *testing.T) {
	conn := NewSSHConnection()
	// Close on a connection that was never opened should not panic or error
	if err := conn.Close(); err != nil {
		t.Errorf("Close on nil connection returned error: %v", err)
	}
}

func TestSSHConnection_CloseMultipleTimes(t *testing.T) {
	conn := NewSSHConnection()
	// Multiple Close calls should be safe
	for i := 0; i < 3; i++ {
		if err := conn.Close(); err != nil {
			t.Errorf("Close call %d returned error: %v", i+1, err)
		}
	}
}

func TestSSHConnection_LocalForwardNotConnected(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	_, err := conn.LocalForward(ctx, 8080, "localhost:80")
	if err == nil {
		t.Error("LocalForward should return error when not connected")
	}
}

func TestSSHConnection_RemoteForwardNotConnected(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	_, err := conn.RemoteForward(ctx, 8080, "localhost:80", "")
	if err == nil {
		t.Error("RemoteForward should return error when not connected")
	}
}

func TestSSHConnection_DynamicForwardNotConnected(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	_, err := conn.DynamicForward(ctx, 1080)
	if err == nil {
		t.Error("DynamicForward should return error when not connected")
	}
}

// TestSSHConnection_DialAttemptsConnectionWithNoAuthMethods は認証メソッドが
// 0 個でも Dial が早期リターンせず TCP 接続を試行することを検証する。
// Tailscale SSH のように none 認証で動作するサーバーをサポートするため必要。
func TestSSHConnection_DialAttemptsConnectionWithNoAuthMethods(t *testing.T) {
	// SSH_AUTH_SOCK を無効化して SSH エージェントを使えなくする
	t.Setenv("SSH_AUTH_SOCK", "")
	// HOME を空ディレクトリに設定してデフォルト鍵ファイルを見つけられなくする
	t.Setenv("HOME", t.TempDir())

	// TCP accept するがデータを送らないサーバー
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)

	host := core.SSHHost{
		Name:     "tailscale-host",
		HostName: "127.0.0.1",
		Port:     addr.Port,
		User:     "testuser",
	}

	conn := NewSSHConnection()
	defer func() { _ = conn.Close() }()

	_, dialErr := conn.Dial(host, nil)
	if dialErr == nil {
		t.Fatal("Dial should return error (handshake fails with dummy server)")
	}

	// 早期リターンのエラーではなく、TCP/SSH レイヤーのエラーであること
	errMsg := dialErr.Error()
	if errMsg == "no authentication methods available for host tailscale-host" {
		t.Error("Dial should not return early with 'no authentication methods available'; " +
			"it should attempt TCP connection to allow none auth")
	}
}

// TestSSHConnection_DialTimeoutOnHangingHandshake は SSH ハンドシェイクが
// ハングした場合にタイムアウトでエラーが返ることを検証する回帰テスト。
func TestSSHConnection_DialTimeoutOnHangingHandshake(t *testing.T) {
	// TCP accept するがデータを送らないサーバー（ハンドシェイクがハングする状況を再現）
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	// accept してコネクションを保持するだけ（何も送らない）
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// コネクションを閉じずに保持（ハンドシェイクがハングする状況）
			defer func() { _ = conn.Close() }()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)

	host := core.SSHHost{
		Name:     "test-hang",
		HostName: "127.0.0.1",
		Port:     addr.Port,
		User:     "testuser",
	}

	conn := NewSSHConnection()
	defer func() { _ = conn.Close() }()

	start := time.Now()
	_, dialErr := conn.Dial(host, nil)
	elapsed := time.Since(start)

	if dialErr == nil {
		t.Fatal("Dial should return error when handshake hangs")
	}

	// 10 秒タイムアウト + 余裕で 15 秒以内に返ること
	if elapsed > 15*time.Second {
		t.Errorf("Dial took too long: %v (expected < 15s)", elapsed)
	}
}

// TestSSHConnection_DialWithProxyCommand は ProxyCommand 経由で接続した場合、
// TCP ダイヤルエラー ("failed to dial") ではなく SSH ハンドシェイクエラー
// ("failed to establish SSH connection") が返ることを検証する。
func TestSSHConnection_DialWithProxyCommand(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())

	host := core.SSHHost{
		Name:         "proxy-test",
		HostName:     "10.255.255.1",
		Port:         22,
		User:         "testuser",
		ProxyCommand: "cat",
	}

	conn := NewSSHConnection()
	defer func() { _ = conn.Close() }()

	_, err := conn.Dial(host, nil)
	if err == nil {
		t.Fatal("Dial should return error")
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "failed to dial") {
		t.Errorf("expected SSH handshake error, got TCP dial error: %s", errMsg)
	}
	if !strings.Contains(errMsg, "failed to establish SSH connection") {
		t.Errorf("expected 'failed to establish SSH connection' in error, got: %s", errMsg)
	}
}

// TestSSHConnection_DialWithProxyCommandPriority は ProxyCommand と ProxyJump の
// 両方が設定されている場合、ProxyCommand が優先されることを検証する。
func TestSSHConnection_DialWithProxyCommandPriority(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())

	host := core.SSHHost{
		Name:         "priority-test",
		HostName:     "10.255.255.1",
		Port:         22,
		User:         "testuser",
		ProxyCommand: "cat",
		ProxyJump:    []string{"jumphost"},
	}

	conn := NewSSHConnection()
	defer func() { _ = conn.Close() }()

	_, err := conn.Dial(host, nil)
	if err == nil {
		t.Fatal("Dial should return error")
	}

	// ProxyCommand が優先されるため、TCP ダイヤルエラーは出ない
	errMsg := err.Error()
	if strings.Contains(errMsg, "failed to dial") {
		t.Errorf("ProxyCommand should take priority over ProxyJump, got TCP dial error: %s", errMsg)
	}
}

// TestSSHConnection_DialStrictHostKeyCheckingNo は StrictHostKeyChecking=no のホストで
// Dial が knownhosts エラーにならないことを検証する回帰テスト。
// known_hosts にエントリがなくても接続試行が knownhosts 起因で失敗しないことを確認する。
func TestSSHConnection_DialStrictHostKeyCheckingNo(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())

	// TCP accept するがデータを送らないサーバー
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)

	host := core.SSHHost{
		Name:                  "strict-no-test",
		HostName:              "127.0.0.1",
		Port:                  addr.Port,
		User:                  "testuser",
		StrictHostKeyChecking: "no",
	}

	conn := NewSSHConnection()
	defer func() { _ = conn.Close() }()

	_, dialErr := conn.Dial(host, nil)
	if dialErr == nil {
		t.Fatal("Dial should return error (handshake fails with dummy server)")
	}

	// knownhosts 起因のエラーではないこと
	errMsg := dialErr.Error()
	if strings.Contains(errMsg, "knownhosts") || strings.Contains(errMsg, "known_hosts") {
		t.Errorf("StrictHostKeyChecking=no should skip host key verification, got: %s", errMsg)
	}
	if strings.Contains(errMsg, "host key") {
		t.Errorf("StrictHostKeyChecking=no should skip host key verification, got: %s", errMsg)
	}
}
