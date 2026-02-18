package ipc

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"golang.org/x/crypto/ssh"
)

// --- Mock implementations ---

type mockSSHManager struct {
	hosts     []core.SSHHost
	loadErr   error
	reloadErr error
	connectFn func(hostName string) error
	disconnFn func(hostName string) error
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
	return m.Connect(hostName)
}

func (m *mockSSHManager) GetPendingAuthHosts() []string { return nil }

func (m *mockSSHManager) Disconnect(hostName string) error {
	if m.disconnFn != nil {
		return m.disconnFn(hostName)
	}
	return nil
}

func (m *mockSSHManager) IsConnected(_ string) bool { return false }
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
	status        DaemonStatusResult
	shutdownFn    func(purge bool) error
	lastPurgeFlag bool
}

func (m *mockDaemonInfo) Status() DaemonStatusResult {
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

	sender, _ := collectingSender()
	broker := NewEventBroker(sender)

	daemon := &mockDaemonInfo{
		status: DaemonStatusResult{
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

// --- Tests ---

func TestHandler_HostList(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "host.list", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	hostList, ok := result.(HostListResult)
	if !ok {
		t.Fatalf("result type = %T, want HostListResult", result)
	}

	if len(hostList.Hosts) != 2 {
		t.Fatalf("hosts count = %d, want 2", len(hostList.Hosts))
	}

	if hostList.Hosts[0].Name != "prod" {
		t.Errorf("hosts[0].Name = %q, want %q", hostList.Hosts[0].Name, "prod")
	}
	if hostList.Hosts[0].State != "connected" {
		t.Errorf("hosts[0].State = %q, want %q", hostList.Hosts[0].State, "connected")
	}
	if hostList.Hosts[1].State != "disconnected" {
		t.Errorf("hosts[1].State = %q, want %q", hostList.Hosts[1].State, "disconnected")
	}
}

func TestHandler_SSHConnect_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, SSHConnectParams{Host: "prod"})
	result, rpcErr := h.Handle("client-1", "ssh.connect", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	connectResult, ok := result.(SSHConnectResult)
	if !ok {
		t.Fatalf("result type = %T, want SSHConnectResult", result)
	}
	if connectResult.Host != "prod" {
		t.Errorf("host = %q, want %q", connectResult.Host, "prod")
	}
	if connectResult.Status != "connected" {
		t.Errorf("status = %q, want %q", connectResult.Status, "connected")
	}
}

func TestHandler_SSHConnect_Error(t *testing.T) {
	h, sshMgr, _, _ := newTestHandler()
	sshMgr.connectFn = func(hostName string) error {
		return fmt.Errorf("host %q not found", hostName)
	}

	params := mustMarshal(t, SSHConnectParams{Host: "nonexistent"})
	_, rpcErr := h.Handle("client-1", "ssh.connect", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != HostNotFound {
		t.Errorf("error code = %d, want %d (HostNotFound)", rpcErr.Code, HostNotFound)
	}
}

func TestHandler_ForwardList(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "forward.list", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	fwdList, ok := result.(ForwardListResult)
	if !ok {
		t.Fatalf("result type = %T, want ForwardListResult", result)
	}

	if len(fwdList.Forwards) != 1 {
		t.Fatalf("forwards count = %d, want 1", len(fwdList.Forwards))
	}
	if fwdList.Forwards[0].Name != "web" {
		t.Errorf("forwards[0].Name = %q, want %q", fwdList.Forwards[0].Name, "web")
	}
	if fwdList.Forwards[0].Type != "local" {
		t.Errorf("forwards[0].Type = %q, want %q", fwdList.Forwards[0].Type, "local")
	}
}

func TestHandler_ForwardAdd_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, ForwardAddParams{
		Name:       "db-tunnel",
		Host:       "prod",
		Type:       "local",
		LocalPort:  5432,
		RemoteHost: "localhost",
		RemotePort: 5432,
	})

	result, rpcErr := h.Handle("client-1", "forward.add", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	addResult, ok := result.(ForwardAddResult)
	if !ok {
		t.Fatalf("result type = %T, want ForwardAddResult", result)
	}
	if addResult.Name != "db-tunnel" {
		t.Errorf("name = %q, want %q", addResult.Name, "db-tunnel")
	}
}

func TestHandler_ForwardAdd_RuleAlreadyExists(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()
	fwdMgr.addErr = fmt.Errorf("rule %q already exists", "web")

	params := mustMarshal(t, ForwardAddParams{
		Name:       "web",
		Host:       "prod",
		Type:       "local",
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	})

	_, rpcErr := h.Handle("client-1", "forward.add", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != RuleAlreadyExists {
		t.Errorf("error code = %d, want %d (RuleAlreadyExists)", rpcErr.Code, RuleAlreadyExists)
	}
}

func TestHandler_SessionList(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "session.list", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	sessionList, ok := result.(SessionListResult)
	if !ok {
		t.Fatalf("result type = %T, want SessionListResult", result)
	}

	if len(sessionList.Sessions) != 1 {
		t.Fatalf("sessions count = %d, want 1", len(sessionList.Sessions))
	}
	if sessionList.Sessions[0].Name != "web" {
		t.Errorf("sessions[0].Name = %q, want %q", sessionList.Sessions[0].Name, "web")
	}
	if sessionList.Sessions[0].Status != "active" {
		t.Errorf("sessions[0].Status = %q, want %q", sessionList.Sessions[0].Status, "active")
	}
	if sessionList.Sessions[0].BytesSent != 1024 {
		t.Errorf("sessions[0].BytesSent = %d, want %d", sessionList.Sessions[0].BytesSent, 1024)
	}
}

func TestHandler_ConfigGet(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "config.get", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgResult, ok := result.(ConfigGetResult)
	if !ok {
		t.Fatalf("result type = %T, want ConfigGetResult", result)
	}

	if cfgResult.SSHConfigPath != "~/.ssh/config" {
		t.Errorf("SSHConfigPath = %q, want %q", cfgResult.SSHConfigPath, "~/.ssh/config")
	}
	if !cfgResult.Reconnect.Enabled {
		t.Error("Reconnect.Enabled should be true")
	}
	if cfgResult.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfgResult.Log.Level, "info")
	}
}

func TestHandler_ConfigUpdate(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	level := "debug"
	file := "/tmp/test.log"
	params := mustMarshal(t, ConfigUpdateParams{
		Log: &LogUpdateInfo{Level: &level, File: &file},
	})

	result, rpcErr := h.Handle("client-1", "config.update", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	updateResult, ok := result.(ConfigUpdateResult)
	if !ok {
		t.Fatalf("result type = %T, want ConfigUpdateResult", result)
	}
	if !updateResult.OK {
		t.Error("OK should be true")
	}

	// 設定が更新されていることを確認
	cfg := cfgMgr.GetConfig()
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.File != "/tmp/test.log" {
		t.Errorf("Log.File = %q, want %q", cfg.Log.File, "/tmp/test.log")
	}
}

func TestHandler_DaemonStatus(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "daemon.status", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	statusResult, ok := result.(DaemonStatusResult)
	if !ok {
		t.Fatalf("result type = %T, want DaemonStatusResult", result)
	}
	if statusResult.PID != 1234 {
		t.Errorf("PID = %d, want %d", statusResult.PID, 1234)
	}
	if statusResult.ConnectedClients != 2 {
		t.Errorf("ConnectedClients = %d, want %d", statusResult.ConnectedClients, 2)
	}
}

func TestHandler_DaemonStatus_NilDaemon(t *testing.T) {
	sender, _ := collectingSender()
	broker := NewEventBroker(sender)
	h := NewHandler(&mockSSHManager{}, &mockForwardManager{}, &mockConfigManager{}, broker, nil)

	_, rpcErr := h.Handle("client-1", "daemon.status", nil)
	if rpcErr == nil {
		t.Fatal("expected RPC error when daemon is nil")
	}
	if rpcErr.Code != InternalError {
		t.Errorf("error code = %d, want %d", rpcErr.Code, InternalError)
	}
}

func TestHandler_DaemonShutdown(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "daemon.shutdown", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	shutdownResult, ok := result.(DaemonShutdownResult)
	if !ok {
		t.Fatalf("result type = %T, want DaemonShutdownResult", result)
	}
	if !shutdownResult.OK {
		t.Error("OK should be true")
	}
}

func TestHandler_EventsSubscribe(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, EventsSubscribeParams{Types: []string{"ssh", "forward"}})
	result, rpcErr := h.Handle("client-1", "events.subscribe", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	subResult, ok := result.(EventsSubscribeResult)
	if !ok {
		t.Fatalf("result type = %T, want EventsSubscribeResult", result)
	}
	if subResult.SubscriptionID == "" {
		t.Error("SubscriptionID should not be empty")
	}
}

func TestHandler_EventsUnsubscribe(t *testing.T) {
	h, _, _, _ := newTestHandler()

	// まず購読を作成
	subParams := mustMarshal(t, EventsSubscribeParams{Types: []string{"ssh"}})
	subResult, _ := h.Handle("client-1", "events.subscribe", subParams)
	subID := subResult.(EventsSubscribeResult).SubscriptionID

	// 購読を解除
	unsubParams := mustMarshal(t, EventsUnsubscribeParams{SubscriptionID: subID})
	result, rpcErr := h.Handle("client-1", "events.unsubscribe", unsubParams)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	unsubResult, ok := result.(EventsUnsubscribeResult)
	if !ok {
		t.Fatalf("result type = %T, want EventsUnsubscribeResult", result)
	}
	if !unsubResult.OK {
		t.Error("OK should be true")
	}

	// 存在しない購読 ID で解除するとエラー
	badParams := mustMarshal(t, EventsUnsubscribeParams{SubscriptionID: "nonexistent"})
	_, rpcErr = h.Handle("client-1", "events.unsubscribe", badParams)
	if rpcErr == nil {
		t.Fatal("expected RPC error for nonexistent subscription")
	}
}

func TestHandler_HostReload(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "host.reload", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	reloadResult, ok := result.(HostReloadResult)
	if !ok {
		t.Fatalf("result type = %T, want HostReloadResult", result)
	}
	if reloadResult.Total != 2 {
		t.Errorf("Total = %d, want 2", reloadResult.Total)
	}
}

func TestHandler_SSHDisconnect_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, SSHDisconnectParams{Host: "prod"})
	result, rpcErr := h.Handle("client-1", "ssh.disconnect", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	discResult, ok := result.(SSHDisconnectResult)
	if !ok {
		t.Fatalf("result type = %T, want SSHDisconnectResult", result)
	}
	if discResult.Status != "disconnected" {
		t.Errorf("status = %q, want %q", discResult.Status, "disconnected")
	}
}

func TestHandler_SSHDisconnect_NotConnected(t *testing.T) {
	h, sshMgr, _, _ := newTestHandler()
	sshMgr.disconnFn = func(hostName string) error {
		return fmt.Errorf("host %q not connected", hostName)
	}

	params := mustMarshal(t, SSHDisconnectParams{Host: "prod"})
	_, rpcErr := h.Handle("client-1", "ssh.disconnect", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != NotConnected {
		t.Errorf("error code = %d, want %d (NotConnected)", rpcErr.Code, NotConnected)
	}
}

func TestHandler_ForwardDelete_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, ForwardDeleteParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "forward.delete", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	deleteResult, ok := result.(ForwardDeleteResult)
	if !ok {
		t.Fatalf("result type = %T, want ForwardDeleteResult", result)
	}
	if !deleteResult.OK {
		t.Error("OK should be true")
	}
}

func TestHandler_ForwardDelete_NotFound(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()
	fwdMgr.deleteErr = fmt.Errorf("rule %q not found", "nonexistent")

	params := mustMarshal(t, ForwardDeleteParams{Name: "nonexistent"})
	_, rpcErr := h.Handle("client-1", "forward.delete", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != RuleNotFound {
		t.Errorf("error code = %d, want %d (RuleNotFound)", rpcErr.Code, RuleNotFound)
	}
}

func TestHandler_ForwardStart_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, ForwardStartParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "forward.start", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	startResult, ok := result.(ForwardStartResult)
	if !ok {
		t.Fatalf("result type = %T, want ForwardStartResult", result)
	}
	if startResult.Status != "active" {
		t.Errorf("status = %q, want %q", startResult.Status, "active")
	}
}

func TestHandler_ForwardStop_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, ForwardStopParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "forward.stop", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	stopResult, ok := result.(ForwardStopResult)
	if !ok {
		t.Fatalf("result type = %T, want ForwardStopResult", result)
	}
	if stopResult.Status != "stopped" {
		t.Errorf("status = %q, want %q", stopResult.Status, "stopped")
	}
}

func TestHandler_SessionGet_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, SessionGetParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "session.get", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	sessionInfo, ok := result.(SessionInfo)
	if !ok {
		t.Fatalf("result type = %T, want SessionInfo", result)
	}
	if sessionInfo.Name != "web" {
		t.Errorf("Name = %q, want %q", sessionInfo.Name, "web")
	}
	if sessionInfo.Status != "active" {
		t.Errorf("Status = %q, want %q", sessionInfo.Status, "active")
	}
}

func TestHandler_SessionGet_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, SessionGetParams{Name: "nonexistent"})
	_, rpcErr := h.Handle("client-1", "session.get", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != RuleNotFound {
		t.Errorf("error code = %d, want %d (RuleNotFound)", rpcErr.Code, RuleNotFound)
	}
}

func TestHandler_EventsSubscribe_InvalidType(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, EventsSubscribeParams{Types: []string{"ssh", "invalid"}})
	_, rpcErr := h.Handle("client-1", "events.subscribe", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error for invalid event type")
	}
	if rpcErr.Code != InvalidParams {
		t.Errorf("error code = %d, want %d (InvalidParams)", rpcErr.Code, InvalidParams)
	}
}

func TestHandler_MethodNotFound(t *testing.T) {
	h, _, _, _ := newTestHandler()

	_, rpcErr := h.Handle("client-1", "nonexistent.method", nil)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != MethodNotFound {
		t.Errorf("error code = %d, want %d (MethodNotFound)", rpcErr.Code, MethodNotFound)
	}
}

func TestHandler_ForwardStopAll(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "forward.stopAll", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	stopAllResult, ok := result.(ForwardStopAllResult)
	if !ok {
		t.Fatalf("result type = %T, want ForwardStopAllResult", result)
	}

	// sessions には Active が 1 件ある
	if stopAllResult.Stopped != 1 {
		t.Errorf("Stopped = %d, want 1", stopAllResult.Stopped)
	}
	if !fwdMgr.stopAllCalled {
		t.Error("StopAllForwards should have been called")
	}
}

func TestHandler_DaemonShutdown_Purge(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, DaemonShutdownParams{Purge: true})
	result, rpcErr := h.Handle("client-1", "daemon.shutdown", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	shutdownResult, ok := result.(DaemonShutdownResult)
	if !ok {
		t.Fatalf("result type = %T, want DaemonShutdownResult", result)
	}
	if !shutdownResult.OK {
		t.Error("OK should be true")
	}

	// handler 内で daemon.Shutdown(true) が呼ばれたことを検証するには
	// newTestHandler で作成した mockDaemonInfo を取り出す必要がある。
	// newTestHandler は daemon を直接返さないため、別途テストを構成する。
}

func TestHandler_DaemonShutdown_PurgeFlag(t *testing.T) {
	sshMgr := &mockSSHManager{}
	fwdMgr := &mockForwardManager{}
	cfgMgr := &mockConfigManager{}
	sender, _ := collectingSender()
	broker := NewEventBroker(sender)
	daemonMock := &mockDaemonInfo{}

	handler := NewHandler(sshMgr, fwdMgr, cfgMgr, broker, daemonMock)

	params := mustMarshal(t, DaemonShutdownParams{Purge: true})
	_, rpcErr := handler.Handle("client-1", "daemon.shutdown", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	if !daemonMock.lastPurgeFlag {
		t.Error("Shutdown should have been called with purge=true")
	}
}

func TestHandler_ForwardAdd_SavesConfig(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	params := mustMarshal(t, ForwardAddParams{
		Name:       "db-tunnel",
		Host:       "prod",
		Type:       "local",
		LocalPort:  5432,
		RemoteHost: "localhost",
		RemotePort: 5432,
	})

	before := cfgMgr.updateCallCount
	_, rpcErr := h.Handle("client-1", "forward.add", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	if cfgMgr.updateCallCount <= before {
		t.Error("forward.add should call UpdateConfig to auto-save rules")
	}

	// 保存されたルールに追加したルールが含まれることを確認
	cfg := cfgMgr.GetConfig()
	found := false
	for _, f := range cfg.Forwards {
		if f.Name == "db-tunnel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("config.Forwards should contain the added rule")
	}
}

func TestHandler_ForwardDelete_SavesConfig(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	params := mustMarshal(t, ForwardDeleteParams{Name: "web"})

	before := cfgMgr.updateCallCount
	_, rpcErr := h.Handle("client-1", "forward.delete", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	if cfgMgr.updateCallCount <= before {
		t.Error("forward.delete should call UpdateConfig to auto-save rules")
	}
}

// --- クレデンシャル認証テスト ---

// mockNotificationSender はテスト用の通知送信モック。
type mockNotificationSender struct {
	mu            sync.Mutex
	notifications []Notification
	clientID      string
}

func (m *mockNotificationSender) SendNotification(clientID string, notification Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientID = clientID
	m.notifications = append(m.notifications, notification)
	return nil
}

func (m *mockNotificationSender) getNotifications() []Notification {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Notification, len(m.notifications))
	copy(cp, m.notifications)
	return cp
}

func TestHandler_CredentialResponse_NoPending(t *testing.T) {
	h, _, _, _ := newTestHandler()
	params, _ := json.Marshal(CredentialResponseParams{
		RequestID: "cr-nonexistent",
		Value:     "secret",
	})

	_, rpcErr := h.Handle("client-1", "credential.response", params)
	if rpcErr == nil {
		t.Fatal("expected error for non-existent credential request")
	}
	if rpcErr.Code != InvalidParams {
		t.Errorf("expected InvalidParams error code, got %d", rpcErr.Code)
	}
}

func TestHandler_CredentialResponse_RoutesToPending(t *testing.T) {
	h, _, _, _ := newTestHandler()

	reqID := "cr-test-1"
	ch := make(chan CredentialResponseParams, 1)
	h.credMu.Lock()
	h.credPending[reqID] = ch
	h.credMu.Unlock()

	params, _ := json.Marshal(CredentialResponseParams{
		RequestID: reqID,
		Value:     "my-password",
	})

	result, rpcErr := h.Handle("client-1", "credential.response", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	credResult, ok := result.(CredentialResponseResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if !credResult.OK {
		t.Error("expected OK=true")
	}

	// チャネルにレスポンスが送信されたことを確認
	select {
	case resp := <-ch:
		if resp.Value != "my-password" {
			t.Errorf("expected value 'my-password', got %q", resp.Value)
		}
		if resp.RequestID != reqID {
			t.Errorf("expected request_id %q, got %q", reqID, resp.RequestID)
		}
	default:
		t.Fatal("expected credential response in channel")
	}
}

func TestHandler_BuildCredentialCallback_SendsNotification(t *testing.T) {
	h, _, _, _ := newTestHandler()
	sender := &mockNotificationSender{}
	h.SetSender(sender)

	cb := h.buildCredentialCallback("client-1", "test-host")
	if cb == nil {
		t.Fatal("callback should not be nil when sender is set")
	}

	// コールバックを goroutine で実行し、レスポンスをシミュレート
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := cb(core.CredentialRequest{
			Type:   core.CredentialPassword,
			Host:   "test-host",
			Prompt: "Password:",
		})
		if err != nil {
			t.Errorf("unexpected callback error: %v", err)
			return
		}
		if resp.Value != "secret-pwd" {
			t.Errorf("expected value 'secret-pwd', got %q", resp.Value)
		}
	}()

	// 通知が送信されるまで待機
	time.Sleep(50 * time.Millisecond)

	notifications := sender.getNotifications()
	if len(notifications) == 0 {
		t.Fatal("expected credential.request notification to be sent")
	}

	notif := notifications[0]
	if notif.Method != "credential.request" {
		t.Errorf("expected method 'credential.request', got %q", notif.Method)
	}

	// 通知の内容を解析して request_id を取得
	var credReq CredentialRequestNotification
	if err := json.Unmarshal(notif.Params, &credReq); err != nil {
		t.Fatalf("failed to unmarshal notification params: %v", err)
	}
	if credReq.Type != "password" {
		t.Errorf("expected type 'password', got %q", credReq.Type)
	}
	if credReq.Host != "test-host" {
		t.Errorf("expected host 'test-host', got %q", credReq.Host)
	}

	// credential.response を送信
	respParams, _ := json.Marshal(CredentialResponseParams{
		RequestID: credReq.RequestID,
		Value:     "secret-pwd",
	})
	if _, err := h.Handle("client-1", "credential.response", respParams); err != nil {
		t.Fatalf("Handle credential.response failed: %v", err)
	}

	// コールバックの完了を待機
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for callback to complete")
	}
}

func TestHandler_BuildCredentialCallback_NilSender(t *testing.T) {
	h, _, _, _ := newTestHandler()
	// sender が nil の場合、コールバックは nil を返す
	cb := h.buildCredentialCallback("client-1", "test-host")
	if cb != nil {
		t.Error("callback should be nil when sender is nil")
	}
}
