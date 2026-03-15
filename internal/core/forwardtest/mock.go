// Package forwardtest は forward パッケージのテスト用モックとヘルパーを提供する。
package forwardtest

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

// MockSSHManager は core.SSHManager のテスト用モック実装。
type MockSSHManager struct {
	mu              sync.RWMutex
	hosts           map[string]core.SSHHost
	connections     map[string]*ssh.Client
	SSHConns        map[string]core.SSHConnection
	connected       map[string]bool
	ConnectErr      error
	ConnectWithCbFn func(hostName string, cb core.CredentialCallback) error
	subscribers     []chan core.SSHEvent
}

// NewMockSSHManager は MockSSHManager を生成する。
func NewMockSSHManager() *MockSSHManager {
	return &MockSSHManager{
		hosts:       make(map[string]core.SSHHost),
		connections: make(map[string]*ssh.Client),
		SSHConns:    make(map[string]core.SSHConnection),
		connected:   make(map[string]bool),
	}
}

func (m *MockSSHManager) LoadHosts() ([]core.SSHHost, error)   { return nil, nil }
func (m *MockSSHManager) ReloadHosts() ([]core.SSHHost, error) { return nil, nil }
func (m *MockSSHManager) GetHosts() []core.SSHHost             { return nil }
func (m *MockSSHManager) GetPendingAuthHosts() []string        { return nil }

func (m *MockSSHManager) GetHost(name string) (*core.SSHHost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if h, ok := m.hosts[name]; ok {
		return &h, nil
	}
	return nil, &core.NotFoundError{Resource: "host", Name: name}
}

func (m *MockSSHManager) Connect(hostName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ConnectErr != nil {
		return m.ConnectErr
	}
	m.connected[hostName] = true
	return nil
}

func (m *MockSSHManager) ConnectWithCallback(hostName string, cb core.CredentialCallback) error {
	if m.ConnectWithCbFn != nil {
		return m.ConnectWithCbFn(hostName, cb)
	}
	return m.Connect(hostName)
}

func (m *MockSSHManager) Disconnect(hostName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connected, hostName)
	return nil
}

func (m *MockSSHManager) IsConnected(hostName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected[hostName]
}

func (m *MockSSHManager) GetConnection(hostName string) (*ssh.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected[hostName] {
		return nil, &core.NotConnectedError{HostName: hostName}
	}
	return m.connections[hostName], nil
}

func (m *MockSSHManager) GetSSHConnection(hostName string) (core.SSHConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected[hostName] {
		return nil, &core.NotConnectedError{HostName: hostName}
	}
	if conn, ok := m.SSHConns[hostName]; ok {
		return conn, nil
	}
	return nil, fmt.Errorf("no SSH connection for %q", hostName)
}

func (m *MockSSHManager) Subscribe() <-chan core.SSHEvent {
	ch := make(chan core.SSHEvent, 16)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

func (m *MockSSHManager) Close() {
	for _, ch := range m.subscribers {
		close(ch)
	}
}

// SetConnected はテスト用にホストを接続状態にする。
func (m *MockSSHManager) SetConnected(hostName string, sshConn core.SSHConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected[hostName] = true
	m.SSHConns[hostName] = sshConn
}

var _ core.SSHManager = (*MockSSHManager)(nil)

// MockSSHConnection は core.SSHConnection のテスト用モック実装。
type MockSSHConnection struct {
	mu      sync.Mutex
	DialErr error
	Client  *ssh.Client
	Closed  bool
	Alive   bool

	KeepAliveF      func(ctx context.Context, interval time.Duration)
	LocalForwardF   func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)
	RemoteForwardF  func(ctx context.Context, remotePort int, localAddr string, remoteBindAddr string) (net.Listener, error)
	DynamicForwardF func(ctx context.Context, localPort int) (net.Listener, error)
}

func (m *MockSSHConnection) Dial(_ core.SSHHost, _ core.CredentialCallback) (*ssh.Client, error) {
	if m.DialErr != nil {
		return nil, m.DialErr
	}
	return m.Client, nil
}

func (m *MockSSHConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Closed = true
	return nil
}

func (m *MockSSHConnection) IsAlive() bool { return m.Alive }

func (m *MockSSHConnection) LocalForward(ctx context.Context, p int, addr string) (net.Listener, error) {
	if m.LocalForwardF != nil {
		return m.LocalForwardF(ctx, p, addr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockSSHConnection) RemoteForward(ctx context.Context, p int, addr string, remoteBindAddr string) (net.Listener, error) {
	if m.RemoteForwardF != nil {
		return m.RemoteForwardF(ctx, p, addr, remoteBindAddr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockSSHConnection) DynamicForward(ctx context.Context, p int) (net.Listener, error) {
	if m.DynamicForwardF != nil {
		return m.DynamicForwardF(ctx, p)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockSSHConnection) KeepAlive(ctx context.Context, interval time.Duration) {
	if m.KeepAliveF != nil {
		m.KeepAliveF(ctx, interval)
		return
	}
	<-ctx.Done()
}

var _ core.SSHConnection = (*MockSSHConnection)(nil)

// NewMockConn は MockSSHConnection を生成するヘルパー。
func NewMockConn(local, dynamic bool) *MockSSHConnection {
	c := &MockSSHConnection{Alive: true}
	if local {
		c.LocalForwardF = func(_ context.Context, _ int, _ string) (net.Listener, error) { return NewMockListener(), nil }
	}
	if dynamic {
		c.DynamicForwardF = func(_ context.Context, _ int) (net.Listener, error) { return NewMockListener(), nil }
	}
	return c
}

// MockSOCKS5Dialer は SOCKS5 ダイアルのテスト用モック。
type MockSOCKS5Dialer struct {
	DialF func(n, addr string) (net.Conn, error)
}

func (d *MockSOCKS5Dialer) Dial(n, addr string) (net.Conn, error) {
	if d.DialF != nil {
		return d.DialF(n, addr)
	}
	return nil, fmt.Errorf("not implemented")
}

// MockListener は net.Listener のテスト用モック。
type MockListener struct {
	mu     sync.Mutex
	closed bool
	ConnCh chan net.Conn
}

func NewMockListener() *MockListener { return &MockListener{ConnCh: make(chan net.Conn)} }

func (l *MockListener) Accept() (net.Conn, error) {
	conn, ok := <-l.ConnCh
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}
	return conn, nil
}

func (l *MockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		l.closed = true
		close(l.ConnCh)
	}
	return nil
}

func (l *MockListener) Addr() net.Addr { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0} }

// DrainEvent はイベントチャネルからイベントを1件読み出す。タイムアウトで失敗する。
func DrainEvent(t *testing.T, ch <-chan core.ForwardEvent) core.ForwardEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
		return core.ForwardEvent{}
	}
}

// AssertSessionStatus はセッションのステータスを検証する。
func AssertSessionStatus(t *testing.T, fm core.ForwardManager, name string, want core.SessionStatus) {
	t.Helper()
	session, err := fm.GetSession(name)
	if err != nil {
		t.Fatalf("GetSession(%q) error = %v", name, err)
	}
	if session.Status != want {
		t.Errorf("session %q status = %v, want %v", name, session.Status, want)
	}
}
