package core

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// mockSSHConnection は SSHConnection のテスト用モック。
// forward_test.go などで使用される。
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

func (m *mockSSHConnection) Dial(host SSHHost, cb CredentialCallback) (*ssh.Client, error) {
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

var _ SSHConnection = (*mockSSHConnection)(nil)
