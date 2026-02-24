package forward

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

// --- Mock SSHManager for ForwardManager tests ---

type mockSSHManager struct {
	mu          sync.RWMutex
	hosts       map[string]core.SSHHost
	connections map[string]*ssh.Client
	sshConns    map[string]core.SSHConnection
	connected   map[string]bool
	connectErr  error
	subscribers []chan core.SSHEvent
}

func newMockSSHManager() *mockSSHManager {
	return &mockSSHManager{
		hosts:       make(map[string]core.SSHHost),
		connections: make(map[string]*ssh.Client),
		sshConns:    make(map[string]core.SSHConnection),
		connected:   make(map[string]bool),
	}
}

func (m *mockSSHManager) LoadHosts() ([]core.SSHHost, error)   { return nil, nil }
func (m *mockSSHManager) ReloadHosts() ([]core.SSHHost, error) { return nil, nil }
func (m *mockSSHManager) GetHosts() []core.SSHHost             { return nil }

func (m *mockSSHManager) GetHost(name string) (*core.SSHHost, error) {
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

func (m *mockSSHManager) ConnectWithCallback(hostName string, cb core.CredentialCallback) error {
	return m.Connect(hostName)
}

func (m *mockSSHManager) GetPendingAuthHosts() []string { return nil }

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

func (m *mockSSHManager) GetSSHConnection(hostName string) (core.SSHConnection, error) {
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

func (m *mockSSHManager) Subscribe() <-chan core.SSHEvent {
	ch := make(chan core.SSHEvent, 16)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

func (m *mockSSHManager) Close() {
	for _, ch := range m.subscribers {
		close(ch)
	}
}

// setConnected はホストを接続済みに設定し、mock SSHConnection を登録する。
func (m *mockSSHManager) setConnected(hostName string, sshConn core.SSHConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected[hostName] = true
	m.sshConns[hostName] = sshConn
}

var _ core.SSHManager = (*mockSSHManager)(nil)

// --- Mock SSHConnection ---

type mockSSHConnection struct {
	mu         sync.Mutex
	dialErr    error
	client     *ssh.Client
	closed     bool
	isAlive    bool
	keepAliveF func(ctx context.Context, interval time.Duration)

	localForwardF   func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)
	remoteForwardF  func(ctx context.Context, remotePort int, localAddr string) (net.Listener, error)
	dynamicForwardF func(ctx context.Context, localPort int) (net.Listener, error)
}

func (m *mockSSHConnection) Dial(host core.SSHHost, cb core.CredentialCallback) (*ssh.Client, error) {
	if m.dialErr != nil {
		return nil, m.dialErr
	}
	return m.client, nil
}

func (m *mockSSHConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockSSHConnection) LocalForward(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error) {
	if m.localForwardF != nil {
		return m.localForwardF(ctx, localPort, remoteAddr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) RemoteForward(ctx context.Context, remotePort int, localAddr string) (net.Listener, error) {
	if m.remoteForwardF != nil {
		return m.remoteForwardF(ctx, remotePort, localAddr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) DynamicForward(ctx context.Context, localPort int) (net.Listener, error) {
	if m.dynamicForwardF != nil {
		return m.dynamicForwardF(ctx, localPort)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) IsAlive() bool {
	return m.isAlive
}

func (m *mockSSHConnection) KeepAlive(ctx context.Context, interval time.Duration) {
	if m.keepAliveF != nil {
		m.keepAliveF(ctx, interval)
		return
	}
	// デフォルト: コンテキストがキャンセルされるまでブロック
	<-ctx.Done()
}

var _ core.SSHConnection = (*mockSSHConnection)(nil)

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

// --- Tests ---

func TestForwardManager_AddRule(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	name, err := fm.AddRule(core.ForwardRule{
		Name:       "web",
		Host:       "server1",
		Type:       core.Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}
	if name != "web" {
		t.Errorf("AddRule() name = %q, want %q", name, "web")
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

	name, err := fm.AddRule(core.ForwardRule{
		Host:       "server1",
		Type:       core.Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}
	if name == "" {
		t.Error("auto-generated name should not be empty")
	}

	rules := fm.GetRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].Name != name {
		t.Errorf("rule name = %q, want %q", rules[0].Name, name)
	}
}

func TestForwardManager_AddRule_DuplicateName(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	rule := core.ForwardRule{
		Name:       "web",
		Host:       "server1",
		Type:       core.Local,
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	}
	if _, err := fm.AddRule(rule); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	_, err := fm.AddRule(rule)
	if err == nil {
		t.Fatal("AddRule() should return error for duplicate name")
	}
}

func TestForwardManager_AddRule_EmptyHost(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	_, err := fm.AddRule(core.ForwardRule{
		Name:       "test",
		Type:       core.Local,
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
			_, err := fm.AddRule(core.ForwardRule{
				Name:       "test-" + tt.name,
				Host:       "server1",
				Type:       core.Local,
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

	_, err := fm.AddRule(core.ForwardRule{
		Name:       "test",
		Host:       "server1",
		Type:       core.Local,
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
	_, err := fm.AddRule(core.ForwardRule{
		Name:      "socks",
		Host:      "server1",
		Type:      core.Dynamic,
		LocalPort: 1080,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v (Dynamic should not require remote port)", err)
	}
}

func TestForwardManager_DeleteRule(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	if _, err := fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
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
		if _, err := fm.AddRule(core.ForwardRule{
			Name: name, Host: "server1", Type: core.Dynamic, LocalPort: 1080,
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

	_, _ = fm.AddRule(core.ForwardRule{Name: "web1", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_, _ = fm.AddRule(core.ForwardRule{Name: "web2", Host: "server2", Type: core.Dynamic, LocalPort: 1081})
	_, _ = fm.AddRule(core.ForwardRule{Name: "web3", Host: "server1", Type: core.Dynamic, LocalPort: 1082})

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

	fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
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

func TestForwardManager_AddRule_DefaultRemoteHost(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	// Local タイプで RemoteHost を指定しない場合、"localhost" がデフォルトになる
	_, err := fm.AddRule(core.ForwardRule{
		Name:       "web-local",
		Host:       "server1",
		Type:       core.Local,
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
	_, err = fm.AddRule(core.ForwardRule{
		Name:       "web-remote",
		Host:       "server1",
		Type:       core.Remote,
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
	_, err = fm.AddRule(core.ForwardRule{
		Name:      "socks",
		Host:      "server1",
		Type:      core.Dynamic,
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
