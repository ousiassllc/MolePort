package infra

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

// dialTestServer はテスト用 SSH サーバーに接続した sshConnection を返す共通ヘルパー。
func dialTestServer(t *testing.T, s *testSSHServer, cb core.CredentialCallback, hostMod ...func(*core.SSHHost)) core.SSHConnection {
	t.Helper()
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())

	host := testSSHHost(s)
	for _, mod := range hostMod {
		mod(&host)
	}

	conn := NewSSHConnection()
	t.Cleanup(func() { _ = conn.Close() })

	if _, err := conn.Dial(host, cb); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	return conn
}

func TestSSHConnection_DialSuccess(t *testing.T) {
	s := newTestSSHServer(t)
	dialTestServer(t, s, nil)
}

func TestSSHConnection_DialWithPublicKeyAuth(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		t.Fatalf("failed to create ssh public key: %v", err)
	}

	block, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	s := newTestSSHServer(t, withPublicKeyAuth(sshPubKey))
	dialTestServer(t, s, nil, func(h *core.SSHHost) { h.IdentityFile = keyPath })
}

func TestSSHConnection_DialWithPasswordAuth(t *testing.T) {
	const password = "test-password-123"
	s := newTestSSHServer(t, withPasswordAuth(password))
	cb := func(_ core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Value: password}, nil
	}
	dialTestServer(t, s, cb)
}

func TestSSHConnection_IsAliveAfterDial(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)
	if !conn.IsAlive() {
		t.Error("IsAlive should return true after successful Dial")
	}
}

func TestSSHConnection_CloseDisconnects(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)
	if err := conn.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if conn.IsAlive() {
		t.Error("IsAlive should return false after Close")
	}
}

func TestSSHConnection_KeepAliveReturnsOnCancel(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		conn.KeepAlive(ctx, 50*time.Millisecond)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("KeepAlive did not return after context cancellation")
	}
}

func TestSSHConnection_KeepAliveReturnsOnDisconnect(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		conn.KeepAlive(ctx, 50*time.Millisecond)
		close(done)
	}()

	_ = s.ln.Close()
	_ = conn.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("KeepAlive did not return after disconnect")
	}
}
