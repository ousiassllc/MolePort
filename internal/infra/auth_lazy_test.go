//go:build !windows

package infra

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/ousiassllc/moleport/internal/core"
)

const testPassphrase = "test-passphrase"

// lazyAuthEnv はホームディレクトリとエージェントを隔離したテスト環境。
type lazyAuthEnv struct {
	home        string
	passphrases atomic.Int32 // パスフレーズ要求の回数
	passwords   atomic.Int32 // パスワード要求の回数
}

func newLazyAuthEnv(t *testing.T) *lazyAuthEnv {
	t.Helper()
	env := &lazyAuthEnv{home: t.TempDir()}
	t.Setenv("HOME", env.home)
	t.Setenv("SSH_AUTH_SOCK", "")
	if err := os.MkdirAll(filepath.Join(env.home, ".ssh"), 0o700); err != nil {
		t.Fatalf("failed to create .ssh: %v", err)
	}
	return env
}

func (e *lazyAuthEnv) writeKey(t *testing.T, name string, pem []byte) string {
	t.Helper()
	path := filepath.Join(e.home, ".ssh", name)
	if err := os.WriteFile(path, pem, 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return path
}

// callback はパスフレーズ・パスワードの要求を数えて固定値で応答するコールバック。
func (e *lazyAuthEnv) callback() core.CredentialCallback {
	return func(req core.CredentialRequest) (core.CredentialResponse, error) {
		switch req.Type {
		case core.CredentialPassphrase:
			e.passphrases.Add(1)
			return core.CredentialResponse{Value: testPassphrase}, nil
		case core.CredentialPassword:
			e.passwords.Add(1)
			return core.CredentialResponse{Value: "pw"}, nil
		}
		return core.CredentialResponse{Cancelled: true}, nil
	}
}

func (e *lazyAuthEnv) dial(t *testing.T, s *testSSHServer, cb core.CredentialCallback, identityFiles ...string) error {
	t.Helper()
	host := testSSHHost(s)
	host.IdentityFiles = identityFiles
	conn := NewSSHConnection()
	t.Cleanup(func() { _ = conn.Close() })
	_, err := conn.Dial(host, cb)
	return err
}

// publicKeyOf は PEM（非暗号化）の秘密鍵から公開鍵を取り出す。
func publicKeyOf(t *testing.T, unencryptedPEM []byte) ssh.PublicKey {
	t.Helper()
	signer, err := ssh.ParsePrivateKey(unencryptedPEM)
	if err != nil {
		t.Fatalf("failed to parse key: %v", err)
	}
	return signer.PublicKey()
}

// startTestAgent は signer を保持する ssh-agent を起動し、SSH_AUTH_SOCK に設定する。
func startTestAgent(t *testing.T, key ed25519.PrivateKey) ssh.PublicKey {
	t.Helper()
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: key}); err != nil {
		t.Fatalf("failed to add key to agent: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("failed to listen on agent socket: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				_ = agent.ServeAgent(keyring, c)
			}()
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sock)
	return signer.PublicKey()
}

func TestDial_AgentAuthSucceeds_DoesNotPromptPassphrase(t *testing.T) {
	env := newLazyAuthEnv(t)
	_, encrypted := generateTestKey(t)
	env.writeKey(t, "id_ed25519", encrypted) // 認証には使わない暗号化された既定の鍵

	_, agentKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	s := newTestSSHServer(t, withPublicKeyAuth(startTestAgent(t, agentKey)))

	if err := env.dial(t, s, env.callback()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	if got := env.passphrases.Load(); got != 0 {
		t.Errorf("passphrase requested %d times, want 0 (agent authenticated)", got)
	}
}

func TestDial_IdentityFileAuthSucceeds_DoesNotPromptOtherDefaultKeys(t *testing.T) {
	env := newLazyAuthEnv(t)
	idPEM, _ := generateTestKey(t)
	idPath := env.writeKey(t, "custom_key", idPEM)
	_, otherEncrypted := generateTestKey(t)
	env.writeKey(t, "id_ed25519", otherEncrypted) // 使われない暗号化された既定の鍵
	s := newTestSSHServer(t, withPublicKeyAuth(publicKeyOf(t, idPEM)))

	if err := env.dial(t, s, env.callback(), idPath); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	if got := env.passphrases.Load(); got != 0 {
		t.Errorf("passphrase requested %d times, want 0 (IdentityFile authenticated)", got)
	}
}

func TestDial_EncryptedKeyOnly_PromptsPassphrase(t *testing.T) {
	env := newLazyAuthEnv(t)
	plain, encrypted := generateTestKey(t)
	env.writeKey(t, "id_ed25519", encrypted)
	s := newTestSSHServer(t, withPublicKeyAuth(publicKeyOf(t, plain)))

	if err := env.dial(t, s, env.callback()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	if got := env.passphrases.Load(); got != 1 {
		t.Errorf("passphrase requested %d times, want 1", got)
	}
}

func TestDial_PasswordOnlyServer_DoesNotPromptPassphrase(t *testing.T) {
	env := newLazyAuthEnv(t)
	_, encrypted := generateTestKey(t)
	env.writeKey(t, "id_ed25519", encrypted)
	s := newTestSSHServer(t, withPasswordAuth("pw"))

	if err := env.dial(t, s, env.callback()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	if got := env.passphrases.Load(); got != 0 {
		t.Errorf("passphrase requested %d times, want 0 (server does not accept publickey)", got)
	}
	if got := env.passwords.Load(); got != 1 {
		t.Errorf("password requested %d times, want 1", got)
	}
}

func TestDial_WrongKeyThenNextKey(t *testing.T) {
	// 先頭の鍵がサーバーに受理されない場合も、後続の鍵で認証できる（段階的に試行される）
	env := newLazyAuthEnv(t)
	wrongPEM, _ := generateTestKey(t)
	wrongPath := env.writeKey(t, "wrong", wrongPEM)
	goodPEM, _ := generateTestKey(t)
	goodPath := env.writeKey(t, "good", goodPEM)
	s := newTestSSHServer(t, withPublicKeyAuth(publicKeyOf(t, goodPEM)))

	if err := env.dial(t, s, nil, wrongPath, goodPath); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
}
