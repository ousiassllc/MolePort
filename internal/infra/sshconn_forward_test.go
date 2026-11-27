package infra

import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// handleSilentSSHConn はグローバルリクエスト（keepalive 等）に応答しない SSH サーバーの接続ハンドラ。
// 無音で途絶したネットワーク（RST も FIN も来ない状態）を模擬する。
func handleSilentSSHConn(netConn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, cfg)
	if err != nil {
		_ = netConn.Close()
		return
	}
	defer func() { _ = sshConn.Close() }()

	go func() {
		for range reqs { //nolint:revive // 応答せず読み捨てる
		}
	}()
	for newCh := range chans {
		_ = newCh.Reject(ssh.Prohibited, "not supported")
	}
}

func dialSilentServer(t *testing.T) *sshConnection {
	t.Helper()
	s := startTestSSHServer(t, handleSilentSSHConn)
	conn, ok := dialTestServer(t, s, nil).(*sshConnection)
	if !ok {
		t.Fatal("unexpected SSHConnection implementation")
	}
	return conn
}

func TestSSHConnection_KeepAliveReturnsOnResponseTimeout(t *testing.T) {
	conn := dialSilentServer(t)

	done := make(chan struct{})
	go func() {
		conn.KeepAlive(context.Background(), 100*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("KeepAlive did not return although the server never responds")
	}
	if conn.getClient() != nil {
		t.Error("connection should be closed after keepalive timeout")
	}
}

func TestSSHConnection_CheckAliveTimeout(t *testing.T) {
	conn := dialSilentServer(t)

	if conn.checkAlive(context.Background(), 100*time.Millisecond) {
		t.Error("checkAlive should return false when the server does not respond")
	}
	if conn.getClient() != nil {
		t.Error("connection should be closed after timeout")
	}
}

func TestSSHConnection_CheckAliveReturnsOnCancel(t *testing.T) {
	conn := dialSilentServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan bool, 1)
	go func() { result <- conn.checkAlive(ctx, time.Minute) }()

	cancel()
	select {
	case alive := <-result:
		if alive {
			t.Error("checkAlive should return false after cancellation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("checkAlive did not return after context cancellation")
	}
	if conn.getClient() == nil {
		t.Error("cancellation must not close the connection")
	}
}

func TestSSHConnection_LocalForwardSuccess(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := conn.LocalForward(ctx, 0, "localhost:80")
	if err != nil {
		t.Fatalf("LocalForward failed: %v", err)
	}
	if ln == nil {
		t.Fatal("LocalForward returned nil listener")
	}
	if ln.Addr().String() == "" {
		t.Error("listener address should not be empty")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	if _, err = ln.Accept(); err == nil {
		t.Error("Accept should fail after context cancellation")
	}
}

func TestSSHConnection_DynamicForwardSuccess(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := conn.DynamicForward(ctx, 0)
	if err != nil {
		t.Fatalf("DynamicForward failed: %v", err)
	}
	if ln == nil {
		t.Fatal("DynamicForward returned nil listener")
	}
	if ln.Addr().String() == "" {
		t.Error("listener address should not be empty")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	if _, err = ln.Accept(); err == nil {
		t.Error("Accept should fail after context cancellation")
	}
}

func TestSSHConnection_RemoteForwardSuccess(t *testing.T) {
	s := newTestSSHServerWithTCPIPForward(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := conn.RemoteForward(ctx, 0, "localhost:80", "")
	if err != nil {
		t.Fatalf("RemoteForward failed: %v", err)
	}
	if ln == nil {
		t.Fatal("RemoteForward returned nil listener")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	if _, err = ln.Accept(); err == nil {
		t.Error("Accept should fail after context cancellation")
	}
}
