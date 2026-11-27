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
func blockingDial(started, release chan struct{}) *mockSSHConnection {
	return &mockSSHConnection{
		isAlive: true,
		dialF: func(core.SSHHost) (*cryptossh.Client, error) {
			close(started)
			<-release
			return nil, nil
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

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	recvWithTimeout(t, ch, what)
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

func TestSSHManager_Connect_DisconnectDuringDial(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	mock := blockingDial(started, release)
	sm := newDialTestManager(t, func() core.SSHConnection { return mock }, core.ReconnectConfig{})

	result := make(chan error, 1)
	go func() { result <- sm.Connect("server1") }()
	waitClosed(t, started, "Dial start")

	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	close(release)

	err := recvWithTimeout(t, result, "Connect result")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Connect() error = %v, want context.Canceled", err)
	}
	if sm.IsConnected("server1") {
		t.Error("server1 must not be connected after Disconnect during Dial")
	}
	if host, _ := sm.GetHost("server1"); host.State != core.Disconnected {
		t.Errorf("host state = %v, want %v", host.State, core.Disconnected)
	}
	if !mock.isClosed() {
		t.Error("connection established during Dial must be closed")
	}
}

func TestSSHManager_Connect_CloseDuringDial(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	mock := blockingDial(started, release)
	sm := newDialTestManager(t, func() core.SSHConnection { return mock }, core.ReconnectConfig{})

	result := make(chan error, 1)
	go func() { result <- sm.Connect("server1") }()
	waitClosed(t, started, "Dial start")

	sm.Close()
	close(release)

	err := recvWithTimeout(t, result, "Connect result")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Connect() error = %v, want context.Canceled", err)
	}
	if sm.IsConnected("server1") {
		t.Error("server1 must not be connected after Close during Dial")
	}
	if !mock.isClosed() {
		t.Error("connection established during Dial must be closed")
	}
}

func TestSSHManager_Connect_DialErrorAfterDisconnect_KeepsDisconnected(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	mock := &mockSSHConnection{
		dialF: func(core.SSHHost) (*cryptossh.Client, error) {
			close(started)
			<-release
			return nil, errors.New("connection refused")
		},
	}
	sm := newDialTestManager(t, func() core.SSHConnection { return mock }, core.ReconnectConfig{})

	result := make(chan error, 1)
	go func() { result <- sm.Connect("server1") }()
	waitClosed(t, started, "Dial start")

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
	reconnectConn := blockingDial(started, release)
	calls := 0
	factory := func() core.SSHConnection {
		calls++
		if calls == 1 {
			// 最初の接続は KeepAlive がすぐに戻る = 切断検出
			return &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
		}
		return reconnectConn
	}
	sm := newDialTestManager(t, factory, core.ReconnectConfig{
		Enabled:      true,
		MaxRetries:   3,
		InitialDelay: core.Duration{Duration: time.Millisecond},
		MaxDelay:     core.Duration{Duration: time.Millisecond},
	})
	events := sm.Subscribe()

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	waitClosed(t, started, "reconnect Dial start")

	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	close(release)

	// 再接続の Dial 結果が破棄され、接続が閉じられるのを待つ
	deadline := time.After(dialTestTimeout)
	for !reconnectConn.isClosed() {
		select {
		case <-deadline:
			t.Fatal("connection established by reconnect must be closed after Disconnect")
		case <-time.After(time.Millisecond):
		}
	}

	if sm.IsConnected("server1") {
		t.Error("reconnect must not override Disconnect")
	}
	if host, _ := sm.GetHost("server1"); host.State != core.Disconnected {
		t.Errorf("host state = %v, want %v", host.State, core.Disconnected)
	}
	// 切断済みとして通知される（Connected / Error で上書きされない）
	var last core.SSHEventType
drain:
	for {
		select {
		case ev := <-events:
			last = ev.Type
		default:
			break drain
		}
	}
	if last != core.SSHEventDisconnected {
		t.Errorf("last event = %v, want %v", last, core.SSHEventDisconnected)
	}
}
