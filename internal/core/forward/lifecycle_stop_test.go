package forward

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_StopForward_NotActive(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080,
	})

	// アクティブでないルールの停止はエラーにならない
	if err := fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}
}

func TestForwardManager_StopAllForwards(t *testing.T) {
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

	fm.StartForward("fwd1")
	fm.StartForward("fwd2")

	if err := fm.StopAllForwards(); err != nil {
		t.Fatalf("StopAllForwards() error = %v", err)
	}

	sessions := fm.GetAllSessions()
	for _, s := range sessions {
		if s.Status != core.Stopped {
			t.Errorf("session %q status = %v, want %v", s.Rule.Name, s.Status, core.Stopped)
		}
	}
}

func TestForwardManager_DeleteRule_StopsActive(t *testing.T) {
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

	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	fm.StartForward("web")

	if err := fm.DeleteRule("web"); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}

	rules := fm.GetRules()
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0 after delete", len(rules))
	}
}

func TestForwardManager_Close(t *testing.T) {
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

	events := fm.Subscribe()

	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	fm.StartForward("web")

	// drain started event
	select {
	case <-events:
	case <-time.After(time.Second):
	}

	fm.Close()

	// チャネルが閉じられていること（StopAllForwards の stopped イベントを先にドレインする）
	for {
		_, ok := <-events
		if !ok {
			break
		}
	}
}

func TestForwardManager_StartForward_ListenerError(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := &mockSSHConnection{
		client:  nil,
		isAlive: true,
		localForwardF: func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error) {
			return nil, fmt.Errorf("address already in use")
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)

	fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	err := fm.StartForward("web")
	if err == nil {
		t.Fatal("StartForward() should return error when listener fails")
	}
}

func TestForwardManager_StopForward_ClosesListener(t *testing.T) {
	sm := newMockSSHManager()
	ml := newMockListener()
	mockConn := &mockSSHConnection{
		client:  nil,
		isAlive: true,
		dynamicForwardF: func(ctx context.Context, localPort int) (net.Listener, error) {
			return ml, nil
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)

	fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	fm.StartForward("web")
	fm.StopForward("web")

	ml.mu.Lock()
	closed := ml.closed
	ml.mu.Unlock()

	if !closed {
		t.Error("StopForward should close the listener")
	}
}
