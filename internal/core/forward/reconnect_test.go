package forward

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_MarkReconnecting(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := newMockLocalAndDynamicDefaultConn()
	sm.setConnected("server1", mockConn)
	sm.setConnected("server2", mockConn)
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	_, _ = fm.AddRule(core.ForwardRule{Name: "socks", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_, _ = fm.AddRule(core.ForwardRule{Name: "other", Host: "server2", Type: core.Dynamic, LocalPort: 1081})
	_ = fm.StartForward("web", nil)
	_ = fm.StartForward("socks", nil)
	_ = fm.StartForward("other", nil)
	events := fm.Subscribe()

	fm.MarkReconnecting("server1")

	reconnecting := make(map[string]bool)
	for range 2 {
		ev := drainEvent(t, events)
		if ev.Type != core.ForwardEventReconnecting {
			t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventReconnecting)
		}
		reconnecting[ev.RuleName] = true
	}
	if !reconnecting["web"] || !reconnecting["socks"] {
		t.Errorf("expected reconnecting events for web and socks, got %v", reconnecting)
	}
	assertSessionStatus(t, fm, "web", core.SessionReconnecting)
	assertSessionStatus(t, fm, "socks", core.SessionReconnecting)
	assertSessionStatus(t, fm, "other", core.Active)
	fm.Close()
}

func TestForwardManager_RestoreForwards(t *testing.T) {
	sm := newMockSSHManager()
	callCount := 0
	mockConn := &mockSSHConnection{
		isAlive: true,
		localForwardF: func(_ context.Context, _ int, _ string) (net.Listener, error) {
			callCount++
			return newMockListener(), nil
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	_ = fm.StartForward("web", nil)
	events := fm.Subscribe()
	fm.MarkReconnecting("server1")
	drainEvent(t, events)

	results := fm.RestoreForwards("server1")
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !results[0].OK {
		t.Errorf("result OK = false, want true, error = %q", results[0].Error)
	}
	if results[0].RuleName != "web" {
		t.Errorf("result RuleName = %q, want %q", results[0].RuleName, "web")
	}
	ev := drainEvent(t, events)
	if ev.Type != core.ForwardEventRestored {
		t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventRestored)
	}
	if ev.Session == nil {
		t.Fatal("event session should not be nil")
	}
	if ev.Session.Status != core.Active {
		t.Errorf("session status = %v, want %v", ev.Session.Status, core.Active)
	}
	if ev.Session.ReconnectCount != 1 {
		t.Errorf("reconnect count = %d, want 1", ev.Session.ReconnectCount)
	}
	assertSessionStatus(t, fm, "web", core.Active)
	session, _ := fm.GetSession("web")
	if session.ReconnectCount != 1 {
		t.Errorf("reconnect count = %d, want 1", session.ReconnectCount)
	}
	fm.Close()
}

func TestForwardManager_RestoreForwards_Error(t *testing.T) {
	sm := newMockSSHManager()
	callCount := 0
	mockConn := &mockSSHConnection{
		isAlive: true,
		localForwardF: func(_ context.Context, _ int, _ string) (net.Listener, error) {
			callCount++
			if callCount == 1 {
				return newMockListener(), nil
			}
			return nil, fmt.Errorf("address already in use")
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	_ = fm.StartForward("web", nil)
	events := fm.Subscribe()
	fm.MarkReconnecting("server1")
	drainEvent(t, events)

	results := fm.RestoreForwards("server1")
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].OK {
		t.Error("result OK = true, want false")
	}
	if results[0].Error == "" {
		t.Error("result Error should not be empty")
	}
	ev := drainEvent(t, events)
	if ev.Type != core.ForwardEventError {
		t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventError)
	}
	assertSessionStatus(t, fm, "web", core.SessionError)
	fm.Close()
}

func TestForwardManager_FailReconnecting(t *testing.T) {
	sm := newMockSSHManager()
	sm.setConnected("server1", newMockLocalDefaultConn())
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	_ = fm.StartForward("web", nil)
	events := fm.Subscribe()
	fm.MarkReconnecting("server1")
	drainEvent(t, events)

	fm.FailReconnecting("server1")
	ev := drainEvent(t, events)
	if ev.Type != core.ForwardEventError {
		t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventError)
	}
	if ev.Session == nil {
		t.Fatal("event session should not be nil")
	}
	if ev.Session.Status != core.SessionError {
		t.Errorf("session status = %v, want %v", ev.Session.Status, core.SessionError)
	}
	assertSessionStatus(t, fm, "web", core.SessionError)
	session, _ := fm.GetSession("web")
	if session.LastError == "" {
		t.Error("session LastError should not be empty")
	}
	fm.Close()
}
