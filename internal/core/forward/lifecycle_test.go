package forward

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/forwardtest"
)

func TestForwardManager_StartForward_RuleNotFound(t *testing.T) {
	if err := NewForwardManager(context.Background(), forwardtest.NewMockSSHManager()).StartForward("nonexistent", nil); err == nil {
		t.Fatal("StartForward() should return error for nonexistent rule")
	}
}

func TestForwardManager_StartForward_ConnectError(t *testing.T) {
	sm := forwardtest.NewMockSSHManager()
	sm.ConnectErr = fmt.Errorf("connection refused")
	fm := NewForwardManager(context.Background(), sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	if err := fm.StartForward("web", nil); err == nil {
		t.Fatal("StartForward() should return error when SSH connect fails")
	}
}

// TestForwardManager_StartForward_UsesCallbackForConnect は Issue #20 の回帰テスト:
// コールバック付き StartForward が ConnectWithCallback を使用することを検証する。
func TestForwardManager_StartForward_UsesCallbackForConnect(t *testing.T) {
	sm := forwardtest.NewMockSSHManager()
	mockConn := forwardtest.NewMockConn(true, false)
	sm.ConnectErr = fmt.Errorf("authentication required: no authentication methods available")
	var receivedCb core.CredentialCallback
	sm.ConnectWithCbFn = func(hostName string, cb core.CredentialCallback) error {
		receivedCb = cb
		sm.SetConnected(hostName, mockConn)
		return nil
	}
	fm := NewForwardManager(context.Background(), sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	cb := func(_ core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Value: "password123"}, nil
	}
	if err := fm.StartForward("web", cb); err != nil {
		t.Fatalf("StartForward() with callback should succeed, got error: %v", err)
	}
	if receivedCb == nil {
		t.Fatal("ConnectWithCallback should have received a non-nil callback")
	}
	fm.Close()
}

func TestForwardManager_StartForward_Local(t *testing.T) {
	sm := forwardtest.NewMockSSHManager()
	sm.SetConnected("server1", forwardtest.NewMockConn(true, false))
	fm := NewForwardManager(context.Background(), sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	events := fm.Subscribe()
	if err := fm.StartForward("web", nil); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}
	ev := forwardtest.DrainEvent(t, events)
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
	forwardtest.AssertSessionStatus(t, fm, "web", core.Active)
	if err := fm.StartForward("web", nil); err == nil {
		t.Fatal("StartForward() should return error for already active forward")
	}
	if err := fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}
	ev = forwardtest.DrainEvent(t, events)
	if ev.Type != core.ForwardEventStopped {
		t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventStopped)
	}
	forwardtest.AssertSessionStatus(t, fm, "web", core.Stopped)
}

// TestForwardManager_StartForward_ConcurrentSameRule は並行呼び出しで重複リスナーが作成されないことを検証する。
func TestForwardManager_StartForward_ConcurrentSameRule(t *testing.T) {
	sm := forwardtest.NewMockSSHManager()
	sm.ConnectWithCbFn = func(hostName string, _ core.CredentialCallback) error {
		sm.SetConnected(hostName, forwardtest.NewMockConn(true, false))
		return nil
	}
	fm := NewForwardManager(context.Background(), sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make([]error, goroutines)
	for i := range goroutines {
		go func() {
			defer wg.Done()
			errs[i] = fm.StartForward("web", nil)
		}()
	}
	wg.Wait()
	var successCount int
	for _, err := range errs {
		if err == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
	fm.Close()
}

func TestForwardManager_StartForward_RemoteAndDynamic(t *testing.T) {
	tests := []struct {
		name     string
		rule     core.ForwardRule
		mockConn *forwardtest.MockSSHConnection
	}{
		{
			name: "Remote",
			rule: core.ForwardRule{
				Name: "remote-web", Host: "server1", Type: core.Remote, LocalPort: 3000, RemoteHost: "0.0.0.0", RemotePort: 80,
			},
			mockConn: &forwardtest.MockSSHConnection{
				Alive: true,
				RemoteForwardF: func(_ context.Context, _ int, _ string, _ string) (net.Listener, error) {
					return forwardtest.NewMockListener(), nil
				},
			},
		},
		{
			name:     "Dynamic",
			rule:     core.ForwardRule{Name: "socks", Host: "server1", Type: core.Dynamic, LocalPort: 1080},
			mockConn: forwardtest.NewMockConn(false, true),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := forwardtest.NewMockSSHManager()
			sm.SetConnected("server1", tt.mockConn)
			fm := NewForwardManager(context.Background(), sm)
			_, _ = fm.AddRule(tt.rule)
			if err := fm.StartForward(tt.rule.Name, nil); err != nil {
				t.Fatalf("StartForward() error = %v", err)
			}
			forwardtest.AssertSessionStatus(t, fm, tt.rule.Name, core.Active)
			fm.Close()
		})
	}
}
