package forward

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_StopForward_NotActive(t *testing.T) {
	fm := NewForwardManager(newMockSSHManager())
	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	// アクティブでないルールの停止はエラーにならない
	if err := fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}
}

func TestForwardManager_StopAllForwards(t *testing.T) {
	sm := newMockSSHManager()
	sm.setConnected("server1", newMockConn(false, true))
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{Name: "fwd1", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_, _ = fm.AddRule(core.ForwardRule{Name: "fwd2", Host: "server1", Type: core.Dynamic, LocalPort: 1081})
	_ = fm.StartForward("fwd1", nil)
	_ = fm.StartForward("fwd2", nil)
	if err := fm.StopAllForwards(); err != nil {
		t.Fatalf("StopAllForwards() error = %v", err)
	}
	for _, s := range fm.GetAllSessions() {
		if s.Status != core.Stopped {
			t.Errorf("session %q status = %v, want %v", s.Rule.Name, s.Status, core.Stopped)
		}
	}
}

func TestForwardManager_DeleteRule_StopsActive(t *testing.T) {
	sm := newMockSSHManager()
	sm.setConnected("server1", newMockConn(false, true))
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_ = fm.StartForward("web", nil)
	if err := fm.DeleteRule("web"); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}
	if rules := fm.GetRules(); len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0 after delete", len(rules))
	}
}

func TestForwardManager_Close(t *testing.T) {
	sm := newMockSSHManager()
	sm.setConnected("server1", newMockConn(false, true))
	fm := NewForwardManager(sm)
	events := fm.Subscribe()
	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_ = fm.StartForward("web", nil)
	drainEvent(t, events) // drain started event
	fm.Close()
	for range events { // drain until channel closed
	}
}

func TestForwardManager_StartForward_ListenerError(t *testing.T) {
	sm := newMockSSHManager()
	sm.setConnected("server1", &mockSSHConnection{
		isAlive: true,
		localForwardF: func(_ context.Context, _ int, _ string) (net.Listener, error) {
			return nil, fmt.Errorf("address already in use")
		},
	})
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	if err := fm.StartForward("web", nil); err == nil {
		t.Fatal("StartForward() should return error when listener fails")
	}
}

func TestForwardManager_StopForward_ClosesListener(t *testing.T) {
	sm := newMockSSHManager()
	ml := newMockListener()
	sm.setConnected("server1", &mockSSHConnection{
		isAlive:         true,
		dynamicForwardF: func(_ context.Context, _ int) (net.Listener, error) { return ml, nil },
	})
	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_ = fm.StartForward("web", nil)
	_ = fm.StopForward("web")
	ml.mu.Lock()
	closed := ml.closed
	ml.mu.Unlock()
	if !closed {
		t.Error("StopForward should close the listener")
	}
}
