package daemon

import (
	"fmt"
	"sync"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// --- Mock: SSHManager (minimal) ---

var _ core.SSHManager = (*mockSSHManagerForState)(nil)

type mockSSHManagerForState struct {
	subscribeCh chan core.SSHEvent
	hosts       []core.SSHHost
}

func (m *mockSSHManagerForState) LoadHosts() ([]core.SSHHost, error)   { return m.hosts, nil }
func (m *mockSSHManagerForState) ReloadHosts() ([]core.SSHHost, error) { return m.hosts, nil }
func (m *mockSSHManagerForState) GetHosts() []core.SSHHost             { return m.hosts }
func (m *mockSSHManagerForState) GetHost(string) (*core.SSHHost, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mockSSHManagerForState) Connect(string) error { return nil }
func (m *mockSSHManagerForState) ConnectWithCallback(string, core.CredentialCallback) error {
	return nil
}
func (m *mockSSHManagerForState) GetPendingAuthHosts() []string { return nil }
func (m *mockSSHManagerForState) Disconnect(string) error       { return nil }
func (m *mockSSHManagerForState) IsConnected(string) bool       { return false }
func (m *mockSSHManagerForState) GetConnection(string) (*cryptossh.Client, error) {
	return nil, fmt.Errorf("not connected")
}
func (m *mockSSHManagerForState) GetSSHConnection(string) (core.SSHConnection, error) {
	return nil, fmt.Errorf("not connected")
}
func (m *mockSSHManagerForState) Subscribe() <-chan core.SSHEvent {
	if m.subscribeCh != nil {
		return m.subscribeCh
	}
	return make(chan core.SSHEvent, 1)
}
func (m *mockSSHManagerForState) Close() {}

// newBrokerStub は通知を無視するテスト用 EventBroker を返す。
func newBrokerStub() *ipc.EventBroker {
	return ipc.NewEventBroker(func(string, protocol.Notification) error { return nil })
}

// --- Tests: logRestoreSummary ---

func TestLogRestoreSummary(t *testing.T) {
	tests := []struct {
		name    string
		results []core.ForwardRestoreResult
	}{
		{"empty", nil},
		{"all_success", []core.ForwardRestoreResult{{RuleName: "web", OK: true}, {RuleName: "db", OK: true}}},
		{"mixed", []core.ForwardRestoreResult{
			{RuleName: "web", OK: true}, {RuleName: "db", OK: false, Error: "refused"}, {RuleName: "api", OK: true},
		}},
		{"all_failed", []core.ForwardRestoreResult{{RuleName: "web", OK: false, Error: "timeout"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDaemonForStateTest(&core.Config{}, &mockForwardManagerForState{})
			d.logRestoreSummary("myhost", tt.results) // パニックしないことを確認
		})
	}
}

// --- Tests: restoreState ---

func TestRestoreState(t *testing.T) {
	t.Run("auto_restore_disabled", func(t *testing.T) {
		fwd := &mockForwardManagerForState{}
		d := newDaemonForStateTest(&core.Config{Session: core.SessionConfig{AutoRestore: false}}, fwd)
		d.restoreState()
		if len(fwd.startCalls) != 0 {
			t.Fatalf("startCalls = %v, want none", fwd.startCalls)
		}
	})
	t.Run("load_state_error", func(t *testing.T) {
		fwd := &mockForwardManagerForState{}
		d := newDaemonForStateTest(&core.Config{Session: core.SessionConfig{AutoRestore: true}}, fwd)
		d.restoreState()
		if len(fwd.startCalls) != 0 {
			t.Fatalf("startCalls = %v, want none", fwd.startCalls)
		}
	})
	t.Run("successful_restore", func(t *testing.T) {
		fwd := &mockForwardManagerForState{}
		cfgMgr := &mockConfigManagerForState{
			config: &core.Config{Session: core.SessionConfig{AutoRestore: true}},
			loadStateFn: func() (*core.State, error) {
				return &core.State{ActiveForwards: []core.ForwardRule{{Name: "web"}, {Name: "db"}}}, nil
			},
		}
		d := newDaemonForStateTestFull(cfgMgr, fwd)
		d.restoreState()
		if len(fwd.startCalls) != 2 || fwd.startCalls[0] != "web" || fwd.startCalls[1] != "db" {
			t.Fatalf("startCalls = %v, want [web db]", fwd.startCalls)
		}
	})
	t.Run("restore_with_start_failure", func(t *testing.T) {
		fwd := &mockForwardManagerForState{startForwardFn: func(n string, _ core.CredentialCallback) error {
			if n == "web" {
				return fmt.Errorf("fail")
			}
			return nil
		}}
		cfgMgr := &mockConfigManagerForState{
			config: &core.Config{Session: core.SessionConfig{AutoRestore: true}},
			loadStateFn: func() (*core.State, error) {
				return &core.State{ActiveForwards: []core.ForwardRule{{Name: "web"}, {Name: "db"}}}, nil
			},
		}
		d := newDaemonForStateTestFull(cfgMgr, fwd)
		d.restoreState()
		if len(fwd.startCalls) != 2 {
			t.Fatalf("startCalls = %d, want 2", len(fwd.startCalls))
		}
	})
}

// --- Tests: saveState ---

func TestSaveState(t *testing.T) {
	t.Run("saves_only_active", func(t *testing.T) {
		var saved *core.State
		fwd := &mockForwardManagerForState{getAllSessionsFn: func() []core.ForwardSession {
			return []core.ForwardSession{
				{Status: core.Active, Rule: core.ForwardRule{Name: "web"}},
				{Status: core.Stopped, Rule: core.ForwardRule{Name: "db"}},
				{Status: core.Active, Rule: core.ForwardRule{Name: "api"}},
			}
		}}
		cfgMgr := &mockConfigManagerForState{
			config: &core.Config{}, saveStateFn: func(s *core.State) error { saved = s; return nil },
		}
		d := newDaemonForStateTestFull(cfgMgr, fwd)
		d.saveState()
		if saved == nil || len(saved.ActiveForwards) != 2 {
			t.Fatalf("ActiveForwards count = %v, want 2", saved)
		}
		if saved.ActiveForwards[0].Name != "web" || saved.ActiveForwards[1].Name != "api" {
			t.Errorf("names = [%s,%s], want [web,api]", saved.ActiveForwards[0].Name, saved.ActiveForwards[1].Name)
		}
	})
	t.Run("save_error_no_panic", func(t *testing.T) {
		fwd := &mockForwardManagerForState{getAllSessionsFn: func() []core.ForwardSession {
			return []core.ForwardSession{{Status: core.Active, Rule: core.ForwardRule{Name: "web"}}}
		}}
		cfgMgr := &mockConfigManagerForState{
			config: &core.Config{}, saveStateFn: func(*core.State) error { return fmt.Errorf("disk full") },
		}
		d := newDaemonForStateTestFull(cfgMgr, fwd)
		d.saveState()
	})
	t.Run("no_active_sessions", func(t *testing.T) {
		var saved *core.State
		fwd := &mockForwardManagerForState{getAllSessionsFn: func() []core.ForwardSession {
			return []core.ForwardSession{{Status: core.Stopped, Rule: core.ForwardRule{Name: "db"}}}
		}}
		cfgMgr := &mockConfigManagerForState{
			config: &core.Config{}, saveStateFn: func(s *core.State) error { saved = s; return nil },
		}
		d := newDaemonForStateTestFull(cfgMgr, fwd)
		d.saveState()
		if saved == nil || len(saved.ActiveForwards) != 0 {
			t.Fatalf("ActiveForwards = %v, want empty", saved)
		}
	})
}

// --- Tests: startEventRouting ---

func TestStartEventRouting(t *testing.T) {
	t.Run("reconnecting_then_connected_restores", func(t *testing.T) {
		sshCh, fwdCh := make(chan core.SSHEvent, 4), make(chan core.ForwardEvent, 1)
		var mu sync.Mutex
		var restoredHost string
		fwd := &mockForwardManagerForState{subscribeCh: fwdCh, restoreForwardsFn: func(h string) []core.ForwardRestoreResult {
			mu.Lock()
			restoredHost = h
			mu.Unlock()
			return []core.ForwardRestoreResult{{RuleName: "web", OK: true}}
		}}
		d := &Daemon{sshMgr: &mockSSHManagerForState{subscribeCh: sshCh}, fwdMgr: fwd, broker: newBrokerStub()}
		d.startEventRouting()
		sshCh <- core.SSHEvent{Type: core.SSHEventReconnecting, HostName: "h1"}
		sshCh <- core.SSHEvent{Type: core.SSHEventConnected, HostName: "h1"}
		close(sshCh)
		close(fwdCh)
		d.wg.Wait()
		fwd.mu.Lock()
		if len(fwd.markReconnectingCalls) != 1 || fwd.markReconnectingCalls[0] != "h1" {
			t.Errorf("markReconnecting = %v, want [h1]", fwd.markReconnectingCalls)
		}
		fwd.mu.Unlock()
		mu.Lock()
		if restoredHost != "h1" {
			t.Errorf("restoredHost = %q, want h1", restoredHost)
		}
		mu.Unlock()
	})
	t.Run("reconnecting_then_error_fails", func(t *testing.T) {
		sshCh, fwdCh := make(chan core.SSHEvent, 4), make(chan core.ForwardEvent, 1)
		fwd := &mockForwardManagerForState{subscribeCh: fwdCh}
		d := &Daemon{sshMgr: &mockSSHManagerForState{subscribeCh: sshCh}, fwdMgr: fwd, broker: newBrokerStub()}
		d.startEventRouting()
		sshCh <- core.SSHEvent{Type: core.SSHEventReconnecting, HostName: "h1"}
		sshCh <- core.SSHEvent{Type: core.SSHEventError, HostName: "h1"}
		close(sshCh)
		close(fwdCh)
		d.wg.Wait()
		fwd.mu.Lock()
		if len(fwd.failReconnectingCalls) != 1 || fwd.failReconnectingCalls[0] != "h1" {
			t.Errorf("failReconnecting = %v, want [h1]", fwd.failReconnectingCalls)
		}
		fwd.mu.Unlock()
	})
	t.Run("connected_without_reconnecting_no_restore", func(t *testing.T) {
		sshCh, fwdCh := make(chan core.SSHEvent, 2), make(chan core.ForwardEvent, 1)
		restored := false
		fwd := &mockForwardManagerForState{subscribeCh: fwdCh, restoreForwardsFn: func(string) []core.ForwardRestoreResult {
			restored = true
			return nil
		}}
		d := &Daemon{sshMgr: &mockSSHManagerForState{subscribeCh: sshCh}, fwdMgr: fwd, broker: newBrokerStub()}
		d.startEventRouting()
		sshCh <- core.SSHEvent{Type: core.SSHEventConnected, HostName: "h1"}
		close(sshCh)
		close(fwdCh)
		d.wg.Wait()
		if restored {
			t.Error("RestoreForwards called without prior reconnecting")
		}
	})
	t.Run("forward_events_routed", func(t *testing.T) {
		sshCh, fwdCh := make(chan core.SSHEvent, 1), make(chan core.ForwardEvent, 2)
		fwd := &mockForwardManagerForState{subscribeCh: fwdCh}
		d := &Daemon{sshMgr: &mockSSHManagerForState{subscribeCh: sshCh}, fwdMgr: fwd, broker: newBrokerStub()}
		d.startEventRouting()
		fwdCh <- core.ForwardEvent{Type: core.ForwardEventStarted, RuleName: "web"}
		close(sshCh)
		close(fwdCh)
		d.wg.Wait() // パニックしないことを確認
	})
}

// --- Tests: Status ---

func TestStatus(t *testing.T) {
	t.Run("with_active_sessions_and_hosts", func(t *testing.T) {
		fwd := &mockForwardManagerForState{getAllSessionsFn: func() []core.ForwardSession {
			return []core.ForwardSession{{Status: core.Active}, {Status: core.Stopped}, {Status: core.Active}}
		}}
		sshMgr := &mockSSHManagerForState{hosts: []core.SSHHost{
			{Name: "a", State: core.Connected}, {Name: "b", State: core.Disconnected}, {Name: "c", State: core.Connected},
		}}
		d := &Daemon{fwdMgr: fwd, sshMgr: sshMgr, version: "1.0.0", startedAt: time.Now().Add(-time.Hour)}
		s := d.Status()
		if s.ActiveForwards != 2 {
			t.Errorf("ActiveForwards = %d, want 2", s.ActiveForwards)
		}
		if s.ActiveSSHConnections != 2 {
			t.Errorf("ActiveSSHConnections = %d, want 2", s.ActiveSSHConnections)
		}
		if s.Version != "1.0.0" {
			t.Errorf("Version = %q, want 1.0.0", s.Version)
		}
	})
	t.Run("empty_state", func(t *testing.T) {
		d := &Daemon{fwdMgr: &mockForwardManagerForState{}, sshMgr: &mockSSHManagerForState{},
			version: "0.1.0", startedAt: time.Now()}
		s := d.Status()
		if s.ActiveForwards != 0 || s.ActiveSSHConnections != 0 {
			t.Errorf("got forwards=%d ssh=%d, want 0,0", s.ActiveForwards, s.ActiveSSHConnections)
		}
	})
}
