package ssh

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestSSHManager_KeepAliveInterval(t *testing.T) {
	hosts := testHosts()

	t.Run("uses configured interval", func(t *testing.T) {
		var gotInterval time.Duration
		sm := NewSSHManager(
			&mockSSHConfigParser{hosts: hosts},
			func() core.SSHConnection {
				mock := &mockSSHConnection{client: nil, isAlive: true}
				mock.keepAliveF = func(_ context.Context, interval time.Duration) {
					gotInterval = interval
				}
				return mock
			},
			"/fake/ssh/config",
			core.ReconnectConfig{
				Enabled:           false,
				KeepAliveInterval: core.Duration{Duration: 45 * time.Second},
			},
		)
		if _, err := sm.LoadHosts(); err != nil {
			t.Fatalf("LoadHosts() error = %v", err)
		}
		if err := sm.Connect("server1"); err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
		time.Sleep(20 * time.Millisecond)
		if gotInterval != 45*time.Second {
			t.Errorf("KeepAlive interval = %v, want 45s", gotInterval)
		}
		sm.Close()
	})

	t.Run("falls back to default", func(t *testing.T) {
		var gotInterval time.Duration
		sm := NewSSHManager(
			&mockSSHConfigParser{hosts: hosts},
			func() core.SSHConnection {
				mock := &mockSSHConnection{client: nil, isAlive: true}
				mock.keepAliveF = func(_ context.Context, interval time.Duration) {
					gotInterval = interval
				}
				return mock
			},
			"/fake/ssh/config",
			core.ReconnectConfig{Enabled: false},
		)
		if _, err := sm.LoadHosts(); err != nil {
			t.Fatalf("LoadHosts() error = %v", err)
		}
		if err := sm.Connect("server1"); err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
		time.Sleep(20 * time.Millisecond)
		if gotInterval != 30*time.Second {
			t.Errorf("KeepAlive interval = %v, want 30s (default)", gotInterval)
		}
		sm.Close()
	})
}

func TestSSHManager_Connect_Disconnect(t *testing.T) {
	hosts := testHosts()
	mockConn := &mockSSHConnection{client: nil, isAlive: true}

	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return mockConn
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if !sm.IsConnected("server1") {
		t.Error("server1 should be connected")
	}

	// 接続イベントを受信
	select {
	case ev := <-events:
		if ev.Type != core.SSHEventConnected {
			t.Errorf("event type = %v, want %v", ev.Type, core.SSHEventConnected)
		}
		if ev.HostName != "server1" {
			t.Errorf("event host = %q, want %q", ev.HostName, "server1")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for connect event")
	}

	host, _ := sm.GetHost("server1")
	if host.State != core.Connected {
		t.Errorf("host state = %v, want %v", host.State, core.Connected)
	}

	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	if sm.IsConnected("server1") {
		t.Error("server1 should be disconnected")
	}

	// 切断イベントを受信
	select {
	case ev := <-events:
		if ev.Type != core.SSHEventDisconnected {
			t.Errorf("event type = %v, want %v", ev.Type, core.SSHEventDisconnected)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for disconnect event")
	}
}

func TestSSHManager_Connect_AlreadyConnected(t *testing.T) {
	hosts := testHosts()
	callCount := 0
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		callCount++
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// 二回目の接続はスキップされる
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}

	if callCount != 1 {
		t.Errorf("connFactory called %d times, want 1", callCount)
	}
}

func TestSSHManager_Connect_DialError(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return &mockSSHConnection{dialErr: fmt.Errorf("connection refused")}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	err := sm.Connect("server1")
	if err == nil {
		t.Fatal("Connect() should return error on dial failure")
	}

	// エラーイベント
	select {
	case ev := <-events:
		if ev.Type != core.SSHEventError {
			t.Errorf("event type = %v, want %v", ev.Type, core.SSHEventError)
		}
		if ev.Error == nil {
			t.Error("event error should not be nil")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error event")
	}

	host, _ := sm.GetHost("server1")
	if host.State != core.ConnectionError {
		t.Errorf("host state = %v, want %v", host.State, core.ConnectionError)
	}
}

func TestSSHManager_Connect_HostNotFound(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	err := sm.Connect("nonexistent")
	if err == nil {
		t.Fatal("Connect() should return error for nonexistent host")
	}
}

func TestSSHManager_Disconnect_NotConnected(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// 接続していないホストの切断はエラーにならない
	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
}

func TestSSHManager_IsConnected_NotLoaded(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if sm.IsConnected("server1") {
		t.Error("should not be connected before LoadHosts")
	}
}

func TestSSHManager_GetConnection(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// 接続前
	_, err := sm.GetConnection("server1")
	if err == nil {
		t.Fatal("GetConnection() should return error when not connected")
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// 接続後（mock では nil client）
	client, err := sm.GetConnection("server1")
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	// mock では nil
	_ = client
}

func TestSSHManager_GetSSHConnection(t *testing.T) {
	hosts := testHosts()
	mockConn := &mockSSHConnection{client: nil, isAlive: true}
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return mockConn
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// 接続前
	_, err := sm.GetSSHConnection("server1")
	if err == nil {
		t.Fatal("GetSSHConnection() should return error when not connected")
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	conn, err := sm.GetSSHConnection("server1")
	if err != nil {
		t.Fatalf("GetSSHConnection() error = %v", err)
	}
	if conn != mockConn {
		t.Error("GetSSHConnection() returned unexpected connection")
	}
}
