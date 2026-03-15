package forward

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/forwardtest"
)

// setupReconnectTest creates a ForwardManager with a single local rule on "server1",
// starts the forward, subscribes to events, and marks the host as reconnecting.
// It drains the reconnecting event before returning.
func setupReconnectTest(t *testing.T, mockConn *forwardtest.MockSSHConnection) (core.ForwardManager, <-chan core.ForwardEvent) {
	t.Helper()
	sm := forwardtest.NewMockSSHManager()
	sm.SetConnected("server1", mockConn)
	fm := NewForwardManager(context.Background(), sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	_ = fm.StartForward("web", nil)
	events := fm.Subscribe()
	fm.MarkReconnecting("server1")
	forwardtest.DrainEvent(t, events)
	return fm, events
}

func TestForwardManager_MarkReconnecting(t *testing.T) {
	sm := forwardtest.NewMockSSHManager()
	mockConn := forwardtest.NewMockConn(true, true)
	sm.SetConnected("server1", mockConn)
	sm.SetConnected("server2", mockConn)
	fm := NewForwardManager(context.Background(), sm)
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
		ev := forwardtest.DrainEvent(t, events)
		if ev.Type != core.ForwardEventReconnecting {
			t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventReconnecting)
		}
		reconnecting[ev.RuleName] = true
	}
	if !reconnecting["web"] || !reconnecting["socks"] {
		t.Errorf("expected reconnecting events for web and socks, got %v", reconnecting)
	}
	forwardtest.AssertSessionStatus(t, fm, "web", core.SessionReconnecting)
	forwardtest.AssertSessionStatus(t, fm, "socks", core.SessionReconnecting)
	forwardtest.AssertSessionStatus(t, fm, "other", core.Active)
	fm.Close()
}

func TestForwardManager_RestoreForwards(t *testing.T) {
	callCount := 0
	mockConn := &forwardtest.MockSSHConnection{
		Alive: true,
		LocalForwardF: func(_ context.Context, _ int, _ string) (net.Listener, error) {
			callCount++
			return forwardtest.NewMockListener(), nil
		},
	}
	fm, events := setupReconnectTest(t, mockConn)
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
	ev := forwardtest.DrainEvent(t, events)
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
	forwardtest.AssertSessionStatus(t, fm, "web", core.Active)
	session, _ := fm.GetSession("web")
	if session.ReconnectCount != 1 {
		t.Errorf("reconnect count = %d, want 1", session.ReconnectCount)
	}
	fm.Close()
}

func TestForwardManager_RestoreForwards_Error(t *testing.T) {
	callCount := 0
	mockConn := &forwardtest.MockSSHConnection{
		Alive: true,
		LocalForwardF: func(_ context.Context, _ int, _ string) (net.Listener, error) {
			callCount++
			if callCount == 1 {
				return forwardtest.NewMockListener(), nil
			}
			return nil, fmt.Errorf("address already in use")
		},
	}
	fm, events := setupReconnectTest(t, mockConn)
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
	ev := forwardtest.DrainEvent(t, events)
	if ev.Type != core.ForwardEventError {
		t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventError)
	}
	forwardtest.AssertSessionStatus(t, fm, "web", core.SessionError)
	fm.Close()
}

func TestForwardManager_FailReconnecting(t *testing.T) {
	fm, events := setupReconnectTest(t, forwardtest.NewMockConn(true, false))
	fm.FailReconnecting("server1")
	ev := forwardtest.DrainEvent(t, events)
	if ev.Type != core.ForwardEventError {
		t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventError)
	}
	if ev.Session == nil {
		t.Fatal("event session should not be nil")
	}
	if ev.Session.Status != core.SessionError {
		t.Errorf("session status = %v, want %v", ev.Session.Status, core.SessionError)
	}
	forwardtest.AssertSessionStatus(t, fm, "web", core.SessionError)
	session, _ := fm.GetSession("web")
	if session.LastError == "" {
		t.Error("session LastError should not be empty")
	}
	fm.Close()
}
