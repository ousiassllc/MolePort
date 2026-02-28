package daemon

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

// --- Mock: ForwardManager ---

var _ core.ForwardManager = (*mockForwardManagerForState)(nil)

type mockForwardManagerForState struct {
	mu             sync.Mutex
	sessions       map[string]*core.ForwardSession
	startCalls     []string
	startForwardFn func(string, core.CredentialCallback) error
}

func (m *mockForwardManagerForState) AddRule(rule core.ForwardRule) (string, error) {
	return rule.Name, nil
}

func (m *mockForwardManagerForState) DeleteRule(string) error { return nil }

func (m *mockForwardManagerForState) GetRules() []core.ForwardRule { return nil }

func (m *mockForwardManagerForState) GetRulesByHost(string) []core.ForwardRule { return nil }

func (m *mockForwardManagerForState) StartForward(ruleName string, cb core.CredentialCallback) error {
	m.mu.Lock()
	m.startCalls = append(m.startCalls, ruleName)
	fn := m.startForwardFn
	m.mu.Unlock()
	if fn != nil {
		return fn(ruleName, cb)
	}
	return nil
}

func (m *mockForwardManagerForState) StopForward(string) error { return nil }

func (m *mockForwardManagerForState) StopAllForwards() error { return nil }

func (m *mockForwardManagerForState) GetSession(ruleName string) (*core.ForwardSession, error) {
	if s, ok := m.sessions[ruleName]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("rule %q not found", ruleName)
}

func (m *mockForwardManagerForState) GetAllSessions() []core.ForwardSession { return nil }

func (m *mockForwardManagerForState) MarkReconnecting(string) {}

func (m *mockForwardManagerForState) RestoreForwards(string) []core.ForwardRestoreResult {
	return nil
}

func (m *mockForwardManagerForState) FailReconnecting(string) {}

func (m *mockForwardManagerForState) Subscribe() <-chan core.ForwardEvent {
	return make(chan core.ForwardEvent, 1)
}

func (m *mockForwardManagerForState) Close() {}

// --- Mock: ConfigManager ---

var _ core.ConfigManager = (*mockConfigManagerForState)(nil)

type mockConfigManagerForState struct {
	config *core.Config
}

func (m *mockConfigManagerForState) GetConfig() *core.Config { return m.config }

func (m *mockConfigManagerForState) LoadConfig() (*core.Config, error) { return m.config, nil }

func (m *mockConfigManagerForState) SaveConfig(*core.Config) error { return nil }

func (m *mockConfigManagerForState) UpdateConfig(func(*core.Config)) error { return nil }

func (m *mockConfigManagerForState) LoadState() (*core.State, error) {
	return nil, fmt.Errorf("no state")
}

func (m *mockConfigManagerForState) SaveState(*core.State) error { return nil }

func (m *mockConfigManagerForState) DeleteState() error { return nil }

func (m *mockConfigManagerForState) ConfigDir() string { return "" }

// --- Helper ---

func newDaemonForStateTest(cfg *core.Config, fwdMgr core.ForwardManager) *Daemon {
	return &Daemon{
		cfgMgr: &mockConfigManagerForState{config: cfg},
		fwdMgr: fwdMgr,
	}
}

// --- Tests ---

func TestAutoStartForwards(t *testing.T) {
	tests := []struct {
		name           string
		forwards       []core.ForwardRule
		sessions       map[string]*core.ForwardSession
		startForwardFn func(string, core.CredentialCallback) error
		wantCalls      []string
	}{
		{
			name: "starts_auto_connect_rules_only",
			forwards: []core.ForwardRule{
				{Name: "web", Host: "myhost", Type: core.Local, LocalPort: 8080, RemotePort: 80, AutoConnect: true},
				{Name: "db", Host: "myhost", Type: core.Local, LocalPort: 5432, RemotePort: 5432, AutoConnect: false},
				{Name: "api", Host: "myhost", Type: core.Local, LocalPort: 3000, RemotePort: 3000, AutoConnect: true},
			},
			wantCalls: []string{"web", "api"},
		},
		{
			name: "skips_already_active_rules",
			forwards: []core.ForwardRule{
				{Name: "web", Host: "myhost", Type: core.Local, LocalPort: 8080, RemotePort: 80, AutoConnect: true},
				{Name: "api", Host: "myhost", Type: core.Local, LocalPort: 3000, RemotePort: 3000, AutoConnect: true},
			},
			sessions: map[string]*core.ForwardSession{
				"web": {Status: core.Active},
			},
			wantCalls: []string{"api"},
		},
		{
			name: "handles_start_failure_gracefully",
			forwards: []core.ForwardRule{
				{Name: "web", Host: "myhost", Type: core.Local, LocalPort: 8080, RemotePort: 80, AutoConnect: true},
				{Name: "api", Host: "myhost", Type: core.Local, LocalPort: 3000, RemotePort: 3000, AutoConnect: true},
				{Name: "metrics", Host: "myhost", Type: core.Local, LocalPort: 9090, RemotePort: 9090, AutoConnect: true},
			},
			startForwardFn: func(ruleName string, _ core.CredentialCallback) error {
				if ruleName == "web" || ruleName == "metrics" {
					return fmt.Errorf("connection refused")
				}
				return nil
			},
			wantCalls: []string{"web", "api", "metrics"},
		},
		{
			name: "no_auto_connect_rules",
			forwards: []core.ForwardRule{
				{Name: "web", Host: "myhost", Type: core.Local, LocalPort: 8080, RemotePort: 80, AutoConnect: false},
				{Name: "db", Host: "myhost", Type: core.Local, LocalPort: 5432, RemotePort: 5432, AutoConnect: false},
			},
			wantCalls: []string{},
		},
		{
			name:      "empty_forwards_list",
			forwards:  []core.ForwardRule{},
			wantCalls: []string{},
		},
		{
			name: "mixed_skip_and_fail_and_success",
			forwards: []core.ForwardRule{
				{Name: "restored", Host: "myhost", Type: core.Local, LocalPort: 8080, RemotePort: 80, AutoConnect: true},
				{Name: "success", Host: "myhost", Type: core.Local, LocalPort: 3000, RemotePort: 3000, AutoConnect: true},
				{Name: "fail", Host: "myhost", Type: core.Local, LocalPort: 4000, RemotePort: 4000, AutoConnect: true},
				{Name: "also-success", Host: "myhost", Type: core.Local, LocalPort: 5000, RemotePort: 5000, AutoConnect: true},
				{Name: "manual", Host: "myhost", Type: core.Local, LocalPort: 6000, RemotePort: 6000, AutoConnect: false},
			},
			sessions: map[string]*core.ForwardSession{
				"restored": {Status: core.Active},
			},
			startForwardFn: func(ruleName string, _ core.CredentialCallback) error {
				if ruleName == "fail" {
					return fmt.Errorf("connection refused")
				}
				return nil
			},
			wantCalls: []string{"success", "fail", "also-success"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockForwardManagerForState{
				sessions:       tt.sessions,
				startForwardFn: tt.startForwardFn,
			}
			cfg := &core.Config{
				Forwards: tt.forwards,
			}

			d := newDaemonForStateTest(cfg, mock)
			d.autoStartForwards()

			got := mock.startCalls
			if got == nil {
				got = []string{}
			}

			if len(got) != len(tt.wantCalls) {
				t.Fatalf("startCalls length = %d, want %d\n  got:  %v\n  want: %v", len(got), len(tt.wantCalls), got, tt.wantCalls)
			}
			for i := range tt.wantCalls {
				if got[i] != tt.wantCalls[i] {
					t.Errorf("startCalls[%d] = %q, want %q\n  got:  %v\n  want: %v", i, got[i], tt.wantCalls[i], got, tt.wantCalls)
				}
			}
		})
	}
}
