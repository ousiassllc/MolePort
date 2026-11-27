package ssh

import (
	"context"
	"errors"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

const dialTestTimeout = 5 * time.Second

// blockingDial は Dial の開始を started で通知し、release が閉じられるまでブロックする mock を返す。
// dialErr が nil でなければ、解放後にそのエラーで Dial を失敗させる。
func blockingDial(started, release chan struct{}, dialErr error) *mockSSHConnection {
	return &mockSSHConnection{
		isAlive: true,
		dialF: func(core.SSHHost) (*cryptossh.Client, error) {
			close(started)
			<-release
			return nil, dialErr
		},
	}
}

func recvWithTimeout[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(dialTestTimeout):
		t.Fatalf("timeout waiting for %s", what)
		panic("unreachable")
	}
}

func newDialTestManager(t *testing.T, connFactory func() core.SSHConnection, cfg core.ReconnectConfig) core.SSHManager {
	t.Helper()
	sm := NewSSHManager(context.Background(), &mockSSHConfigParser{hosts: testHosts()}, connFactory, "/fake/ssh/config", cfg, nil)
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}
	t.Cleanup(sm.Close)
	return sm
}

// Dial 中に Disconnect / Close されても、確立した接続が登録されず破棄されること。
func TestSSHManager_Connect_CancelledDuringDial(t *testing.T) {
	tests := []struct {
		name   string
		cancel func(sm core.SSHManager)
	}{
		{"disconnect", func(sm core.SSHManager) { _ = sm.Disconnect("server1") }},
		{"close", func(sm core.SSHManager) { sm.Close() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			mock := blockingDial(started, release, nil)
			sm := newDialTestManager(t, func() core.SSHConnection { return mock }, core.ReconnectConfig{})

			result := make(chan error, 1)
			go func() { result <- sm.Connect("server1") }()
			recvWithTimeout(t, started, "Dial start")

			tt.cancel(sm)
			close(release)

			if err := recvWithTimeout(t, result, "Connect result"); !errors.Is(err, context.Canceled) {
				t.Errorf("Connect() error = %v, want context.Canceled", err)
			}
			if sm.IsConnected("server1") {
				t.Error("server1 must not be connected after cancellation during Dial")
			}
			if !mock.isClosed() {
				t.Error("connection established during Dial must be closed")
			}
		})
	}
}

func TestSSHManager_Connect_DialErrorAfterDisconnect_KeepsDisconnected(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	mock := blockingDial(started, release, errors.New("connection refused"))
	sm := newDialTestManager(t, func() core.SSHConnection { return mock }, core.ReconnectConfig{})

	result := make(chan error, 1)
	go func() { result <- sm.Connect("server1") }()
	recvWithTimeout(t, started, "Dial start")

	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	close(release)

	if err := recvWithTimeout(t, result, "Connect result"); err == nil {
		t.Error("Connect() should return the dial error")
	}
	if host, _ := sm.GetHost("server1"); host.State != core.Disconnected {
		t.Errorf("host state = %v, want %v (user Disconnect must win)", host.State, core.Disconnected)
	}
}

func TestSSHManager_Reconnect_DisconnectDuringDial(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	reconnectConn := blockingDial(started, release, nil)
	calls := 0
	sm := newDialTestManager(t, func() core.SSHConnection {
		if calls++; calls == 1 {
			return &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
		}
		return reconnectConn
	}, core.ReconnectConfig{
		Enabled:      true,
		MaxRetries:   3,
		InitialDelay: core.Duration{Duration: time.Millisecond},
		MaxDelay:     core.Duration{Duration: time.Millisecond},
	})
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	recvWithTimeout(t, started, "reconnect Dial start")
	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	close(release)

	// 再接続の Dial 結果が破棄され、接続が閉じられるのを待つ（上限付きポーリング）
	for deadline := time.Now().Add(dialTestTimeout); !reconnectConn.isClosed(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("connection established by reconnect must be closed after Disconnect")
		}
	}
	if sm.IsConnected("server1") {
		t.Error("reconnect must not override Disconnect")
	}
	if host, _ := sm.GetHost("server1"); host.State != core.Disconnected {
		t.Errorf("host state = %v, want %v", host.State, core.Disconnected)
	}
}
