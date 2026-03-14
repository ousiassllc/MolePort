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

type mockSSHManager struct {
	mu              sync.RWMutex
	hosts           map[string]core.SSHHost
	connections     map[string]*ssh.Client
	sshConns        map[string]core.SSHConnection
	connected       map[string]bool
	connectErr      error
	connectWithCbFn func(hostName string, cb core.CredentialCallback) error
	subscribers     []chan core.SSHEvent
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
func (m *mockSSHManager) GetPendingAuthHosts() []string        { return nil }

func (m *mockSSHManager) GetHost(name string) (*core.SSHHost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if h, ok := m.hosts[name]; ok {
		return &h, nil
	}
	return nil, &core.NotFoundError{Resource: "host", Name: name}
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
	if m.connectWithCbFn != nil {
		return m.connectWithCbFn(hostName, cb)
	}
	return m.Connect(hostName)
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
		return nil, &core.NotConnectedError{HostName: hostName}
	}
	return m.connections[hostName], nil
}

func (m *mockSSHManager) GetSSHConnection(hostName string) (core.SSHConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected[hostName] {
		return nil, &core.NotConnectedError{HostName: hostName}
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

func (m *mockSSHManager) setConnected(hostName string, sshConn core.SSHConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected[hostName] = true
	m.sshConns[hostName] = sshConn
}

var _ core.SSHManager = (*mockSSHManager)(nil)

type mockSSHConnection struct {
	mu      sync.Mutex
	dialErr error
	client  *ssh.Client
	closed  bool
	isAlive bool

	keepAliveF      func(ctx context.Context, interval time.Duration)
	localForwardF   func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)
	remoteForwardF  func(ctx context.Context, remotePort int, localAddr string, remoteBindAddr string) (net.Listener, error)
	dynamicForwardF func(ctx context.Context, localPort int) (net.Listener, error)
}

func (m *mockSSHConnection) Dial(_ core.SSHHost, _ core.CredentialCallback) (*ssh.Client, error) {
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

func (m *mockSSHConnection) IsAlive() bool { return m.isAlive }

func (m *mockSSHConnection) LocalForward(ctx context.Context, p int, addr string) (net.Listener, error) {
	if m.localForwardF != nil {
		return m.localForwardF(ctx, p, addr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) RemoteForward(ctx context.Context, p int, addr string, remoteBindAddr string) (net.Listener, error) {
	if m.remoteForwardF != nil {
		return m.remoteForwardF(ctx, p, addr, remoteBindAddr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) DynamicForward(ctx context.Context, p int) (net.Listener, error) {
	if m.dynamicForwardF != nil {
		return m.dynamicForwardF(ctx, p)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) KeepAlive(ctx context.Context, interval time.Duration) {
	if m.keepAliveF != nil {
		m.keepAliveF(ctx, interval)
		return
	}
	<-ctx.Done()
}

var _ core.SSHConnection = (*mockSSHConnection)(nil)

func newMockConn(local, dynamic bool) *mockSSHConnection {
	c := &mockSSHConnection{isAlive: true}
	if local {
		c.localForwardF = func(_ context.Context, _ int, _ string) (net.Listener, error) { return newMockListener(), nil }
	}
	if dynamic {
		c.dynamicForwardF = func(_ context.Context, _ int) (net.Listener, error) { return newMockListener(), nil }
	}
	return c
}

type mockSOCKS5Dialer struct {
	dialF func(n, addr string) (net.Conn, error)
}

func (d *mockSOCKS5Dialer) Dial(n, addr string) (net.Conn, error) {
	if d.dialF != nil {
		return d.dialF(n, addr)
	}
	return nil, fmt.Errorf("not implemented")
}

type mockListener struct {
	mu     sync.Mutex
	closed bool
	connCh chan net.Conn
}

func newMockListener() *mockListener { return &mockListener{connCh: make(chan net.Conn)} }

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

func (l *mockListener) Addr() net.Addr { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0} }

func drainEvent(t *testing.T, ch <-chan core.ForwardEvent) core.ForwardEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
		return core.ForwardEvent{}
	}
}

func assertSessionStatus(t *testing.T, fm core.ForwardManager, name string, want core.SessionStatus) {
	t.Helper()
	session, err := fm.GetSession(name)
	if err != nil {
		t.Fatalf("GetSession(%q) error = %v", name, err)
	}
	if session.Status != want {
		t.Errorf("session %q status = %v, want %v", name, session.Status, want)
	}
}
