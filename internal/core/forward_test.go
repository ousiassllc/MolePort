package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// --- Mock SSHManager for ForwardManager tests ---

type mockSSHManager struct {
	mu          sync.RWMutex
	hosts       map[string]SSHHost
	connections map[string]*ssh.Client
	sshConns    map[string]SSHConnection
	connected   map[string]bool
	connectErr  error
	subscribers []chan SSHEvent
}

func newMockSSHManager() *mockSSHManager {
	return &mockSSHManager{
		hosts:       make(map[string]SSHHost),
		connections: make(map[string]*ssh.Client),
		sshConns:    make(map[string]SSHConnection),
		connected:   make(map[string]bool),
	}
}

func (m *mockSSHManager) LoadHosts() ([]SSHHost, error)   { return nil, nil }
func (m *mockSSHManager) ReloadHosts() ([]SSHHost, error) { return nil, nil }

func (m *mockSSHManager) GetHost(name string) (*SSHHost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if h, ok := m.hosts[name]; ok {
		return &h, nil
	}
	return nil, fmt.Errorf("host %q not found", name)
}

func (m *mockSSHManager) Connect(hostName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected[hostName] = true
	return nil
}

func (m *mockSSHManager) Disconnect(hostName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connected, hostName)
	return nil
}

func (m *mockSSHManager) IsConnected(hostName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected[hostName]
}

func (m *mockSSHManager) GetConnection(hostName string) (*ssh.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected[hostName] {
		return nil, fmt.Errorf("not connected")
	}
	return m.connections[hostName], nil
}

func (m *mockSSHManager) GetSSHConnection(hostName string) (SSHConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected[hostName] {
		return nil, fmt.Errorf("not connected")
	}
	if conn, ok := m.sshConns[hostName]; ok {
		return conn, nil
	}
	return nil, fmt.Errorf("no SSH connection for %q", hostName)
}

func (m *mockSSHManager) Subscribe() <-chan SSHEvent {
	ch := make(chan SSHEvent, 16)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

func (m *mockSSHManager) Close() {
	for _, ch := range m.subscribers {
		close(ch)
	}
}

// setConnected はホストを接続済みに設定し、mock SSHConnection を登録する。
func (m *mockSSHManager) setConnected(hostName string, sshConn SSHConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected[hostName] = true
	m.sshConns[hostName] = sshConn
}

var _ SSHManager = (*mockSSHManager)(nil)

// --- Tests ---

func TestForwardManager_AddRule(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.AddRule(ForwardRule{
		Name:       "web",
		Host:       "server1",
		Type:       Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules := fm.GetRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].Name != "web" {
		t.Errorf("rule name = %q, want %q", rules[0].Name, "web")
	}
	if rules[0].Host != "server1" {
		t.Errorf("rule host = %q, want %q", rules[0].Host, "server1")
	}
}

func TestForwardManager_AddRule_AutoName(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.AddRule(ForwardRule{
		Host:       "server1",
		Type:       Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules := fm.GetRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].Name == "" {
		t.Error("auto-generated name should not be empty")
	}
}

func TestForwardManager_AddRule_DuplicateName(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	rule := ForwardRule{
		Name:       "web",
		Host:       "server1",
		Type:       Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	}
	if err := fm.AddRule(rule); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	err := fm.AddRule(rule)
	if err == nil {
		t.Fatal("AddRule() should return error for duplicate name")
	}
}

func TestForwardManager_AddRule_EmptyHost(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.AddRule(ForwardRule{
		Name:       "test",
		Type:       Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	})
	if err == nil {
		t.Fatal("AddRule() should return error for empty host")
	}
}

func TestForwardManager_AddRule_InvalidPort(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	tests := []struct {
		name      string
		localPort int
		wantErr   bool
	}{
		{"zero port", 0, true},
		{"negative port", -1, true},
		{"too large", 65536, true},
		{"valid min", 1, false},
		{"valid max", 65535, false},
		{"valid mid", 8080, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fm.AddRule(ForwardRule{
				Name:       "test-" + tt.name,
				Host:       "server1",
				Type:       Local,
				LocalPort:  tt.localPort,
				RemoteHost: "localhost",
				RemotePort: 80,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestForwardManager_AddRule_InvalidRemotePort(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.AddRule(ForwardRule{
		Name:       "test",
		Host:       "server1",
		Type:       Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 0,
	})
	if err == nil {
		t.Fatal("AddRule() should return error for invalid remote port")
	}
}

func TestForwardManager_AddRule_DynamicNoRemotePort(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	// Dynamic では RemotePort は不要
	err := fm.AddRule(ForwardRule{
		Name:      "socks",
		Host:      "server1",
		Type:      Dynamic,
		LocalPort: 1080,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v (Dynamic should not require remote port)", err)
	}
}

func TestForwardManager_DeleteRule(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	if err := fm.AddRule(ForwardRule{
		Name: "web", Host: "server1", Type: Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	}); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	if err := fm.DeleteRule("web"); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}

	rules := fm.GetRules()
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0", len(rules))
	}
}

func TestForwardManager_DeleteRule_NotFound(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.DeleteRule("nonexistent")
	if err == nil {
		t.Fatal("DeleteRule() should return error for nonexistent rule")
	}
}

func TestForwardManager_GetRules_Order(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		if err := fm.AddRule(ForwardRule{
			Name: name, Host: "server1", Type: Dynamic, LocalPort: 1080,
		}); err != nil {
			t.Fatalf("AddRule(%q) error = %v", name, err)
		}
	}

	rules := fm.GetRules()
	if len(rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(rules))
	}
	for i, name := range names {
		if rules[i].Name != name {
			t.Errorf("rules[%d].Name = %q, want %q", i, rules[i].Name, name)
		}
	}
}

func TestForwardManager_GetRulesByHost(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	fm.AddRule(ForwardRule{Name: "web1", Host: "server1", Type: Dynamic, LocalPort: 1080})
	fm.AddRule(ForwardRule{Name: "web2", Host: "server2", Type: Dynamic, LocalPort: 1081})
	fm.AddRule(ForwardRule{Name: "web3", Host: "server1", Type: Dynamic, LocalPort: 1082})

	rules := fm.GetRulesByHost("server1")
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	if rules[0].Name != "web1" {
		t.Errorf("rules[0].Name = %q, want %q", rules[0].Name, "web1")
	}
	if rules[1].Name != "web3" {
		t.Errorf("rules[1].Name = %q, want %q", rules[1].Name, "web3")
	}
}

func TestForwardManager_GetRulesByHost_Empty(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	rules := fm.GetRulesByHost("nonexistent")
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0", len(rules))
	}
}

func TestForwardManager_StartForward_RuleNotFound(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	err := fm.StartForward("nonexistent")
	if err == nil {
		t.Fatal("StartForward() should return error for nonexistent rule")
	}
}

func TestForwardManager_StartForward_ConnectError(t *testing.T) {
	sm := newMockSSHManager()
	sm.connectErr = fmt.Errorf("connection refused")
	fm := NewForwardManager(sm)

	fm.AddRule(ForwardRule{
		Name: "web", Host: "server1", Type: Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	err := fm.StartForward("web")
	if err == nil {
		t.Fatal("StartForward() should return error when SSH connect fails")
	}
}

func TestForwardManager_StartForward_Local(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := &mockSSHConnection{
		client:  nil,
		isAlive: true,
		localForwardF: func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error) {
			return newMockListener(), nil
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)

	fm.AddRule(ForwardRule{
		Name: "web", Host: "server1", Type: Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	events := fm.Subscribe()

	if err := fm.StartForward("web"); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}

	// Started イベントを確認
	select {
	case ev := <-events:
		if ev.Type != ForwardEventStarted {
			t.Errorf("event type = %v, want %v", ev.Type, ForwardEventStarted)
		}
		if ev.RuleName != "web" {
			t.Errorf("event rule = %q, want %q", ev.RuleName, "web")
		}
		if ev.Session == nil {
			t.Fatal("event session should not be nil")
		}
		if ev.Session.Status != Active {
			t.Errorf("session status = %v, want %v", ev.Session.Status, Active)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for started event")
	}

	// セッション確認
	session, err := fm.GetSession("web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != Active {
		t.Errorf("session status = %v, want %v", session.Status, Active)
	}

	// 二重起動はエラー
	err = fm.StartForward("web")
	if err == nil {
		t.Fatal("StartForward() should return error for already active forward")
	}

	// 停止
	if err := fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}

	select {
	case ev := <-events:
		if ev.Type != ForwardEventStopped {
			t.Errorf("event type = %v, want %v", ev.Type, ForwardEventStopped)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for stopped event")
	}

	session, err = fm.GetSession("web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != Stopped {
		t.Errorf("session status = %v, want %v", session.Status, Stopped)
	}
}

func TestForwardManager_StartForward_Remote(t *testing.T) {
	sm := newMockSSHManager()
	mockConn := &mockSSHConnection{
		client:  nil,
		isAlive: true,
		remoteForwardF: func(ctx context.Context, remotePort int, localAddr string) (net.Listener, error) {
			return newMockListener(), nil
		},
	}
	sm.setConnected("server1", mockConn)
	fm := NewForwardManager(sm)

	fm.AddRule(ForwardRule{
		Name: "remote-web", Host: "server1", Type: Remote, LocalPort: 3000, RemoteHost: "0.0.0.0", RemotePort: 80,
	})

	if err := fm.StartForward("remote-web"); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}

	session, err := fm.GetSession("remote-web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != Active {
		t.Errorf("session status = %v, want %v", session.Status, Active)
	}

	fm.Close()
}

func TestForwardManager_StartForward_Dynamic(t *testing.T) {
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

	fm.AddRule(ForwardRule{
		Name: "socks", Host: "server1", Type: Dynamic, LocalPort: 1080,
	})

	if err := fm.StartForward("socks"); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}

	session, err := fm.GetSession("socks")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != Active {
		t.Errorf("session status = %v, want %v", session.Status, Active)
	}

	fm.Close()
}

func TestForwardManager_StopForward_NotActive(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	fm.AddRule(ForwardRule{
		Name: "web", Host: "server1", Type: Dynamic, LocalPort: 1080,
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

	fm.AddRule(ForwardRule{Name: "fwd1", Host: "server1", Type: Dynamic, LocalPort: 1080})
	fm.AddRule(ForwardRule{Name: "fwd2", Host: "server1", Type: Dynamic, LocalPort: 1081})

	fm.StartForward("fwd1")
	fm.StartForward("fwd2")

	if err := fm.StopAllForwards(); err != nil {
		t.Fatalf("StopAllForwards() error = %v", err)
	}

	sessions := fm.GetAllSessions()
	for _, s := range sessions {
		if s.Status != Stopped {
			t.Errorf("session %q status = %v, want %v", s.Rule.Name, s.Status, Stopped)
		}
	}
}

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

	fm.AddRule(ForwardRule{
		Name: "web", Host: "server1", Type: Dynamic, LocalPort: 1080,
	})

	session, err := fm.GetSession("web")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != Stopped {
		t.Errorf("session status = %v, want %v", session.Status, Stopped)
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

	fm.AddRule(ForwardRule{Name: "fwd1", Host: "server1", Type: Dynamic, LocalPort: 1080})
	fm.AddRule(ForwardRule{Name: "fwd2", Host: "server1", Type: Dynamic, LocalPort: 1081})

	fm.StartForward("fwd1")

	sessions := fm.GetAllSessions()
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}

	if sessions[0].Status != Active {
		t.Errorf("sessions[0] status = %v, want %v", sessions[0].Status, Active)
	}
	if sessions[1].Status != Stopped {
		t.Errorf("sessions[1] status = %v, want %v", sessions[1].Status, Stopped)
	}

	fm.Close()
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

	fm.AddRule(ForwardRule{Name: "web", Host: "server1", Type: Dynamic, LocalPort: 1080})
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

	fm.AddRule(ForwardRule{Name: "web", Host: "server1", Type: Dynamic, LocalPort: 1080})
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

	fm.AddRule(ForwardRule{Name: "web", Host: "server1", Type: Dynamic, LocalPort: 1080})
	fm.StartForward("web")

	for _, ch := range []<-chan ForwardEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != ForwardEventStarted {
				t.Errorf("event type = %v, want %v", ev.Type, ForwardEventStarted)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}

	fm.Close()
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

	fm.AddRule(ForwardRule{
		Name: "web", Host: "server1", Type: Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})

	err := fm.StartForward("web")
	if err == nil {
		t.Fatal("StartForward() should return error when listener fails")
	}
}

// --- SOCKS5 tests ---

func TestHandleSOCKS5_StagedReads(t *testing.T) {
	// SOCKS5 の段階的読み取りが正しく動作することを検証する。
	// クライアント側・サーバー側を net.Pipe で接続し、SOCKS5 ネゴシエーションを行う。
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			// remote 側もパイプで返す
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// Greeting: VER=5, NMETHODS=1, METHODS=[0x00]
	clientConn.Write([]byte{0x05, 0x01, 0x00})

	// サーバーからの応答を読む
	resp := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, resp); err != nil {
		t.Fatalf("read greeting response: %v", err)
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		t.Fatalf("unexpected greeting response: %v", resp)
	}

	// Request: CONNECT to example.com:80 (domain type)
	domain := "example.com"
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(domain))}
	req = append(req, []byte(domain)...)
	req = append(req, 0x00, 0x50) // port 80
	clientConn.Write(req)

	// Success response
	successResp := make([]byte, 10)
	if _, err := io.ReadFull(clientConn, successResp); err != nil {
		t.Fatalf("read success response: %v", err)
	}
	if successResp[0] != 0x05 || successResp[1] != 0x00 {
		t.Fatalf("unexpected success response: %v", successResp)
	}

	// 正しいアドレスに接続されたことを確認
	select {
	case addr := <-dialedAddr:
		if addr != "example.com:80" {
			t.Errorf("dialed addr = %q, want %q", addr, "example.com:80")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

func TestHandleSOCKS5_IPv4(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// Greeting
	clientConn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	io.ReadFull(clientConn, resp)

	// Request: CONNECT to 192.168.1.1:8080 (IPv4)
	req := []byte{0x05, 0x01, 0x00, 0x01, 192, 168, 1, 1, 0x1F, 0x90} // port 8080
	clientConn.Write(req)

	successResp := make([]byte, 10)
	io.ReadFull(clientConn, successResp)

	select {
	case addr := <-dialedAddr:
		if addr != "192.168.1.1:8080" {
			t.Errorf("dialed addr = %q, want %q", addr, "192.168.1.1:8080")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

func TestHandleSOCKS5_IPv6(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// Greeting
	clientConn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	io.ReadFull(clientConn, resp)

	// Request: CONNECT to [::1]:443 (IPv6)
	req := []byte{0x05, 0x01, 0x00, 0x04}
	ipv6 := net.ParseIP("::1").To16()
	req = append(req, ipv6...)
	req = append(req, 0x01, 0xBB) // port 443
	clientConn.Write(req)

	successResp := make([]byte, 10)
	io.ReadFull(clientConn, successResp)

	select {
	case addr := <-dialedAddr:
		expected := net.JoinHostPort("::1", "443")
		if addr != expected {
			t.Errorf("dialed addr = %q, want %q", addr, expected)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

func TestHandleSOCKS5_NoAuthMethodRejected(t *testing.T) {
	// クライアントが 0x00 (no auth) を含まないメソッドリストを送った場合、
	// サーバーは 0xFF (no acceptable methods) を返すことを検証する。
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	mockDialer := &mockSOCKS5Dialer{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		fm.handleSOCKS5(af, serverConn, mockDialer)
	}()

	// Greeting: VER=5, NMETHODS=1, METHODS=[0x02] (username/password only)
	clientConn.Write([]byte{0x05, 0x01, 0x02})

	resp := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, resp); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp[0] != 0x05 || resp[1] != 0xFF {
		t.Errorf("expected no acceptable methods (0xFF), got %v", resp)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleSOCKS5 did not return after rejection")
	}
}

func TestHandleSOCKS5_FragmentedWrites(t *testing.T) {
	// TCP ストリームで段階的に（1バイトずつ）送信しても正しく処理されることを確認する。
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// 1バイトずつ送信: Greeting
	for _, b := range []byte{0x05, 0x01, 0x00} {
		clientConn.Write([]byte{b})
		time.Sleep(time.Millisecond)
	}

	resp := make([]byte, 2)
	io.ReadFull(clientConn, resp)

	// 1バイトずつ送信: Request (domain "a.b" port 80)
	domain := "a.b"
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(domain))}
	req = append(req, []byte(domain)...)
	req = append(req, 0x00, 0x50)
	for _, b := range req {
		clientConn.Write([]byte{b})
		time.Sleep(time.Millisecond)
	}

	successResp := make([]byte, 10)
	io.ReadFull(clientConn, successResp)

	select {
	case addr := <-dialedAddr:
		if addr != "a.b:80" {
			t.Errorf("dialed addr = %q, want %q", addr, "a.b:80")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

// --- Fix 2: Listener close test ---

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

	fm.AddRule(ForwardRule{Name: "web", Host: "server1", Type: Dynamic, LocalPort: 1080})
	fm.StartForward("web")
	fm.StopForward("web")

	ml.mu.Lock()
	closed := ml.closed
	ml.mu.Unlock()

	if !closed {
		t.Error("StopForward should close the listener")
	}
}

// --- Fix 5: DeleteRule TOCTOU test ---

func TestForwardManager_DeleteRule_Concurrent(t *testing.T) {
	// 同じルールに対する並行 DeleteRule が安全に動作することを確認する。
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

	fm.AddRule(ForwardRule{Name: "web", Host: "server1", Type: Dynamic, LocalPort: 1080})
	fm.StartForward("web")

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			fm.DeleteRule("web")
		}()
	}
	wg.Wait()

	rules := fm.GetRules()
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0 after concurrent delete", len(rules))
	}
}

// --- Fix 6: AddRule RemoteHost default test ---

func TestForwardManager_AddRule_DefaultRemoteHost(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	// Local タイプで RemoteHost を指定しない場合、"localhost" がデフォルトになる
	err := fm.AddRule(ForwardRule{
		Name:       "web-local",
		Host:       "server1",
		Type:       Local,
		LocalPort:  8080,
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules := fm.GetRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].RemoteHost != "localhost" {
		t.Errorf("RemoteHost = %q, want %q", rules[0].RemoteHost, "localhost")
	}

	// Remote タイプでも同様
	err = fm.AddRule(ForwardRule{
		Name:       "web-remote",
		Host:       "server1",
		Type:       Remote,
		LocalPort:  3000,
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules = fm.GetRules()
	if rules[1].RemoteHost != "localhost" {
		t.Errorf("RemoteHost = %q, want %q", rules[1].RemoteHost, "localhost")
	}

	// Dynamic タイプでは RemoteHost はそのまま空
	err = fm.AddRule(ForwardRule{
		Name:      "socks",
		Host:      "server1",
		Type:      Dynamic,
		LocalPort: 1080,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules = fm.GetRules()
	if rules[2].RemoteHost != "" {
		t.Errorf("Dynamic RemoteHost = %q, want empty", rules[2].RemoteHost)
	}
}

// --- Mock SOCKS5 dialer ---

type mockSOCKS5Dialer struct {
	dialF func(n, addr string) (net.Conn, error)
}

func (d *mockSOCKS5Dialer) Dial(n, addr string) (net.Conn, error) {
	if d.dialF != nil {
		return d.dialF(n, addr)
	}
	return nil, fmt.Errorf("not implemented")
}

// --- Mock Listener ---

type mockListener struct {
	mu     sync.Mutex
	closed bool
	connCh chan net.Conn
}

func newMockListener() *mockListener {
	return &mockListener{
		connCh: make(chan net.Conn),
	}
}

func (l *mockListener) Accept() (net.Conn, error) {
	conn, ok := <-l.connCh
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}
	return conn, nil
}

func (l *mockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		l.closed = true
		close(l.connCh)
	}
	return nil
}

func (l *mockListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}
