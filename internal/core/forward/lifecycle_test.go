package forward

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_StartForward_RuleNotFound(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.StartForward("nonexistent", nil)
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

	err := fm.StartForward("web", nil)
	if err == nil {
		t.Fatal("StartForward() should return error when SSH connect fails")
	}
}

// TestForwardManager_StartForward_UsesCallbackForConnect は、
// StartForward にコールバックを渡した場合、未接続ホストへの接続に
// ConnectWithCallback（コールバック付き）が使用されることを検証する。
// これは Issue #20 の回帰テスト: 以前は Connect（コールバックなし）が使用され、
// パスワード認証が必要なホストへの接続が失敗していた。
func TestForwardManager_StartForward_UsesCallbackForConnect(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := newMockLocalDefaultConn()

	// Connect（コールバックなし）は認証エラーを返す
	sm.connectErr = fmt.Errorf("authentication required: no authentication methods available")

	// ConnectWithCallback はコールバック付きなら成功する
	var receivedCb core.CredentialCallback
	sm.connectWithCbFn = func(hostName string, cb core.CredentialCallback) error {
		receivedCb = cb
		sm.mu.Lock()
		sm.connected[hostName] = true
		sm.sshConns[hostName] = mockConn
		sm.mu.Unlock()
		return nil
	}

	fm := NewForwardManager(sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	// コールバック付きで StartForward を呼び出す
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Value: "password123"}, nil
	}
	err := fm.StartForward("web", cb)
	if err != nil {
		t.Fatalf("StartForward() with callback should succeed, got error: %v", err)
	}

	// ConnectWithCallback にコールバックが渡されたことを確認
	if receivedCb == nil {
		t.Fatal("ConnectWithCallback should have received a non-nil callback")
	}

	fm.Close()
}

func TestForwardManager_StartForward_Local(t *testing.T) {
	sm := newMockSSHManager()
	sm.setConnected("server1", newMockLocalDefaultConn())
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	events := fm.Subscribe()

	if err := fm.StartForward("web", nil); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}

	ev := drainEvent(t, events)
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
	assertSessionStatus(t, fm, "web", core.Active)

	if err := fm.StartForward("web", nil); err == nil {
		t.Fatal("StartForward() should return error for already active forward")
	}
	if err := fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}
	ev = drainEvent(t, events)
	if ev.Type != core.ForwardEventStopped {
		t.Errorf("event type = %v, want %v", ev.Type, core.ForwardEventStopped)
	}
	assertSessionStatus(t, fm, "web", core.Stopped)
}

func TestForwardManager_StartForward_RemoteAndDynamic(t *testing.T) {
	tests := []struct {
		name     string
		rule     core.ForwardRule
		mockConn *mockSSHConnection
	}{
		{
			name: "Remote",
			rule: core.ForwardRule{
				Name: "remote-web", Host: "server1", Type: core.Remote, LocalPort: 3000, RemoteHost: "0.0.0.0", RemotePort: 80,
			},
			mockConn: &mockSSHConnection{
				isAlive:        true,
				remoteForwardF: func(_ context.Context, _ int, _ string) (net.Listener, error) { return newMockListener(), nil },
			},
		},
		{
			name:     "Dynamic",
			rule:     core.ForwardRule{Name: "socks", Host: "server1", Type: core.Dynamic, LocalPort: 1080},
			mockConn: newMockDynamicDefaultConn(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := newMockSSHManager()
			sm.setConnected("server1", tt.mockConn)
			fm := NewForwardManager(sm)
			_, _ = fm.AddRule(tt.rule)
			if err := fm.StartForward(tt.rule.Name, nil); err != nil {
				t.Fatalf("StartForward() error = %v", err)
			}
			assertSessionStatus(t, fm, tt.rule.Name, core.Active)
			fm.Close()
		})
	}
}
