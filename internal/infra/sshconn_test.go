package infra

import (
	"context"
	"net"
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
	_, err := conn.RemoteForward(ctx, 8080, "localhost:80")
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

// TestSSHConnection_DialTimeoutOnHangingHandshake は SSH ハンドシェイクが
// ハングした場合にタイムアウトでエラーが返ることを検証する回帰テスト。
func TestSSHConnection_DialTimeoutOnHangingHandshake(t *testing.T) {
	// TCP accept するがデータを送らないサーバー（ハンドシェイクがハングする状況を再現）
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	// accept してコネクションを保持するだけ（何も送らない）
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// コネクションを閉じずに保持（ハンドシェイクがハングする状況）
			defer conn.Close()
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
	defer conn.Close()

	start := time.Now()
	_, dialErr := conn.Dial(host)
	elapsed := time.Since(start)

	if dialErr == nil {
		t.Fatal("Dial should return error when handshake hangs")
	}

	// 10 秒タイムアウト + 余裕で 15 秒以内に返ること
	if elapsed > 15*time.Second {
		t.Errorf("Dial took too long: %v (expected < 15s)", elapsed)
	}
}
