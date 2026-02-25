package forward

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_StartForward_RuleNotFound(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.StartForward("nonexistent")
	if err == nil {
		t.Fatal("StartForward() should return error for nonexistent rule")
	}
}

func TestForwardManager_StartForward_ConnectError(t *testing.T) {
	sm := newMockSSHManager()
	sm.connectErr = fmt.Errorf("connection refused")
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	err := fm.StartForward("web")
	if err == nil {
		t.Fatal("StartForward() should return error when SSH connect fails")
	}
}

func TestForwardManager_StartForward_Local(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := &mockSSHConnection{
		client:  nil,
		isAlive: true,
		localForwardF: func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error) {
			return newMockListener(), nil
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	events := fm.Subscribe()

	if err := fm.StartForward("web"); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}

	// Started イベントを確認
	select {
	case ev := <-events:
		if ev.Type != core.ForwardEventStarted {
			t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventStarted)
		}
		if ev.RuleName != "web" {
			t.Errorf("event rule = %q, want %q", ev.RuleName, "web")
		}
		if ev.Session == nil {
			t.Fatal("event session should not be nil")
		}
		if ev.Session.Status != core.Active {
			t.Errorf("session status = %v, want %v", ev.Session.Status, core.Active)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for started event")
	}

	// セッション確認
	session, err := fm.GetSession("web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != core.Active {
		t.Errorf("session status = %v, want %v", session.Status, core.Active)
	}

	// 二重起動はエラー
	err = fm.StartForward("web")
	if err == nil {
		t.Fatal("StartForward() should return error for already active forward")
	}

	// 停止
	if err := fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}

	select {
	case ev := <-events:
		if ev.Type != core.ForwardEventStopped {
			t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventStopped)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for stopped event")
	}

	session, err = fm.GetSession("web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != core.Stopped {
		t.Errorf("session status = %v, want %v", session.Status, core.Stopped)
	}
}

func TestForwardManager_StartForward_Remote(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := &mockSSHConnection{
		client:  nil,
		isAlive: true,
		remoteForwardF: func(ctx context.Context, remotePort int, localAddr string) (net.Listener, error) {
			return newMockListener(), nil
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{
		Name: "remote-web", Host: "server1", Type: core.Remote, LocalPort: 3000, RemoteHost: "0.0.0.0", RemotePort: 80,
	})

	if err := fm.StartForward("remote-web"); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}

	session, err := fm.GetSession("remote-web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != core.Active {
		t.Errorf("session status = %v, want %v", session.Status, core.Active)
	}

	fm.Close()
}

func TestForwardManager_StartForward_Dynamic(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := &mockSSHConnection{
		client:  nil,
		isAlive: true,
		dynamicForwardF: func(ctx context.Context, localPort int) (net.Listener, error) {
			return newMockListener(), nil
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{
		Name: "socks", Host: "server1", Type: core.Dynamic, LocalPort: 1080,
	})

	if err := fm.StartForward("socks"); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}

	session, err := fm.GetSession("socks")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != core.Active {
		t.Errorf("session status = %v, want %v", session.Status, core.Active)
	}

	fm.Close()
}
