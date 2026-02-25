package handler

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"golang.org/x/crypto/ssh"
)

// --- Mock implementations ---

type mockSSHManager struct {
	hosts           []core.SSHHost
	loadErr         error
	reloadErr       error
	connectFn       func(hostName string) error
	connectWithCbFn func(hostName string, cb core.CredentialCallback) error
	disconnFn       func(hostName string) error
	connected       map[string]bool
}

func (m *mockSSHManager) LoadHosts() ([]core.SSHHost, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.hosts, nil
}

func (m *mockSSHManager) ReloadHosts() ([]core.SSHHost, error) {
	if m.reloadErr != nil {
		return nil, m.reloadErr
	}
	return m.hosts, nil
}

func (m *mockSSHManager) GetHosts() []core.SSHHost {
	return m.hosts
}

func (m *mockSSHManager) GetHost(name string) (*core.SSHHost, error) {
	for _, h := range m.hosts {
		if h.Name == name {
			return &h, nil
		}
	}
	return nil, fmt.Errorf("host %q not found", name)
}

func (m *mockSSHManager) Connect(hostName string) error {
	if m.connectFn != nil {
		return m.connectFn(hostName)
	}
	return nil
}

func (m *mockSSHManager) ConnectWithCallback(hostName string, cb core.CredentialCallback) error {
	if m.connectWithCbFn != nil {
		return m.connectWithCbFn(hostName, cb)
	}
	return m.Connect(hostName)
}

func (m *mockSSHManager) GetPendingAuthHosts() []string { return nil }

func (m *mockSSHManager) Disconnect(hostName string) error {
	if m.disconnFn != nil {
		return m.disconnFn(hostName)
	}
	return nil
}

func (m *mockSSHManager) IsConnected(hostName string) bool {
	if m.connected != nil {
		return m.connected[hostName]
	}
	return false
}
func (m *mockSSHManager) GetConnection(_ string) (*ssh.Client, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockSSHManager) GetSSHConnection(_ string) (core.SSHConnection, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockSSHManager) Subscribe() <-chan core.SSHEvent {
	return make(chan core.SSHEvent)
}
func (m *mockSSHManager) Close() {}

type mockForwardManager struct {
	rules         []core.ForwardRule
	sessions      []core.ForwardSession
	addErr        error
	deleteErr     error
	startErr      error
	stopErr       error
	stopAllErr    error
	stopAllCalled bool
	sessionErr    error
}

func (m *mockForwardManager) AddRule(rule core.ForwardRule) (string, error) {
	if m.addErr != nil {
		return "", m.addErr
	}
	if rule.Name == "" {
		rule.Name = "auto-generated"
	}
	m.rules = append(m.rules, rule)
	return rule.Name, nil
}

func (m *mockForwardManager) DeleteRule(name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *mockForwardManager) GetRules() []core.ForwardRule {
	return m.rules
}

func (m *mockForwardManager) GetRulesByHost(hostName string) []core.ForwardRule {
	var result []core.ForwardRule
	for _, r := range m.rules {
		if r.Host == hostName {
			result = append(result, r)
		}
	}
	return result
}

func (m *mockForwardManager) StartForward(ruleName string) error {
	if m.startErr != nil {
		return m.startErr
	}
	return nil
}

func (m *mockForwardManager) StopForward(ruleName string) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	return nil
}

func (m *mockForwardManager) StopAllForwards() error {
	m.stopAllCalled = true
	return m.stopAllErr
}

func (m *mockForwardManager) GetSession(ruleName string) (*core.ForwardSession, error) {
	if m.sessionErr != nil {
		return nil, m.sessionErr
	}
	for _, s := range m.sessions {
		if s.Rule.Name == ruleName {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("rule %q not found", ruleName)
}

func (m *mockForwardManager) GetAllSessions() []core.ForwardSession {
	return m.sessions
}

func (m *mockForwardManager) Subscribe() <-chan core.ForwardEvent {
	return make(chan core.ForwardEvent)
}

func (m *mockForwardManager) Close() {}

type mockConfigManager struct {
	config          *core.Config
	err             error
	updateCallCount int
}

func (m *mockConfigManager) LoadConfig() (*core.Config, error)    { return m.config, m.err }
func (m *mockConfigManager) SaveConfig(config *core.Config) error { return m.err }
func (m *mockConfigManager) GetConfig() *core.Config {
	if m.config == nil {
		cfg := core.DefaultConfig()
		return &cfg
	}
	return m.config
}
func (m *mockConfigManager) UpdateConfig(fn func(*core.Config)) error {
	m.updateCallCount++
	if m.err != nil {
		return m.err
	}
	if m.config == nil {
		cfg := core.DefaultConfig()
		m.config = &cfg
	}
	fn(m.config)
	return nil
}
func (m *mockConfigManager) LoadState() (*core.State, error) { return &core.State{}, nil }
func (m *mockConfigManager) SaveState(_ *core.State) error   { return nil }
func (m *mockConfigManager) DeleteState() error              { return nil }
func (m *mockConfigManager) ConfigDir() string               { return "/tmp/moleport" }

type mockDaemonInfo struct {
	status        protocol.DaemonStatusResult
	shutdownFn    func(purge bool) error
	lastPurgeFlag bool
}

func (m *mockDaemonInfo) Status() protocol.DaemonStatusResult {
	return m.status
}

func (m *mockDaemonInfo) Shutdown(purge bool) error {
	m.lastPurgeFlag = purge
	if m.shutdownFn != nil {
		return m.shutdownFn(purge)
	}
	return nil
}

// --- Test helpers ---

func newTestHandler() (*Handler, *mockSSHManager, *mockForwardManager, *mockConfigManager) {
	sshMgr := &mockSSHManager{
		hosts: []core.SSHHost{
			{Name: "prod", HostName: "prod.example.com", Port: 22, User: "deploy", State: core.Connected},
			{Name: "staging", HostName: "staging.example.com", Port: 22, User: "deploy", State: core.Disconnected},
		},
	}

	fwdMgr := &mockForwardManager{
		rules: []core.ForwardRule{
			{Name: "web", Host: "prod", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
		},
		sessions: []core.ForwardSession{
			{
				ID:          "web-123",
				Rule:        core.ForwardRule{Name: "web", Host: "prod", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
				Status:      core.Active,
				ConnectedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				BytesSent:   1024,
			},
		},
	}

	cfgMgr := &mockConfigManager{}

	sender := func(_ string, _ protocol.Notification) error { return nil }
	broker := ipc.NewEventBroker(sender)

	daemon := &mockDaemonInfo{
		status: protocol.DaemonStatusResult{
			PID:              1234,
			StartedAt:        "2025-01-01T00:00:00Z",
			Uptime:           "1h0m0s",
			ConnectedClients: 2,
		},
	}

	handler := NewHandler(sshMgr, fwdMgr, cfgMgr, broker, daemon)
	return handler, sshMgr, fwdMgr, cfgMgr
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
