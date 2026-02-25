package forward

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_GetSession_NotFound(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	_, err := fm.GetSession("nonexistent")
	if err == nil {
		t.Fatal("GetSession() should return error for nonexistent rule")
	}
}

func TestForwardManager_GetSession_Inactive(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080,
	})

	session, err := fm.GetSession("web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != core.Stopped {
		t.Errorf("session status = %v, want %v", session.Status, core.Stopped)
	}
	if session.Rule.Name != "web" {
		t.Errorf("session rule name = %q, want %q", session.Rule.Name, "web")
	}
}

func TestForwardManager_GetAllSessions(t *testing.T) {
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

	_, _ = fm.AddRule(core.ForwardRule{Name: "fwd1", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_, _ = fm.AddRule(core.ForwardRule{Name: "fwd2", Host: "server1", Type: core.Dynamic, LocalPort: 1081})

	_ = fm.StartForward("fwd1", nil)

	sessions := fm.GetAllSessions()
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}

	if sessions[0].Status != core.Active {
		t.Errorf("sessions[0] status = %v, want %v", sessions[0].Status, core.Active)
	}
	if sessions[1].Status != core.Stopped {
		t.Errorf("sessions[1] status = %v, want %v", sessions[1].Status, core.Stopped)
	}

	fm.Close()
}

func TestForwardManager_Subscribe_MultipleSubscribers(t *testing.T) {
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

	ch1 := fm.Subscribe()
	ch2 := fm.Subscribe()

	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_ = fm.StartForward("web", nil)

	for _, ch := range []<-chan core.ForwardEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != core.ForwardEventStarted {
				t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventStarted)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}

	fm.Close()
}
