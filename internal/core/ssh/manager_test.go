package ssh

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

// --- Mock SSHConfigParser ---

type mockSSHConfigParser struct {
	hosts []core.SSHHost
	err   error
}

func (m *mockSSHConfigParser) Parse(configPath string) ([]core.SSHHost, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([]core.SSHHost, len(m.hosts))
	copy(result, m.hosts)
	return result, nil
}

// --- Mock SSHConnection ---

type mockSSHConnection struct {
	mu         sync.Mutex
	dialErr    error
	client     *cryptossh.Client
	closed     bool
	isAlive    bool
	keepAliveF func(ctx context.Context, interval time.Duration)

	localForwardF   func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)
	remoteForwardF  func(ctx context.Context, remotePort int, localAddr string) (net.Listener, error)
	dynamicForwardF func(ctx context.Context, localPort int) (net.Listener, error)
}

func (m *mockSSHConnection) Dial(host core.SSHHost, cb core.CredentialCallback) (*cryptossh.Client, error) {
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

// インターフェース適合チェック
var _ core.SSHConfigParser = (*mockSSHConfigParser)(nil)
var _ core.SSHConnection = (*mockSSHConnection)(nil)

// --- Helpers ---

func testHosts() []core.SSHHost {
	return []core.SSHHost{
		{Name: "server1", HostName: "192.168.1.1", Port: 22, User: "user1", State: core.Disconnected},
		{Name: "server2", HostName: "192.168.1.2", Port: 2222, User: "user2", State: core.Disconnected},
	}
}

func newTestSSHManager(hosts []core.SSHHost, connFactory func() core.SSHConnection) core.SSHManager {
	parser := &mockSSHConfigParser{hosts: hosts}
	return NewSSHManager(context.Background(), parser, connFactory, "/fake/ssh/config", core.ReconnectConfig{
		Enabled:      false,
		MaxRetries:   3,
		InitialDelay: core.Duration{Duration: 10 * time.Millisecond},
		MaxDelay:     core.Duration{Duration: 50 * time.Millisecond},
	}, nil)
}
