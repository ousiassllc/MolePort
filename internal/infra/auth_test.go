package infra

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestExpandTilde_Exported(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"~/.ssh/config", filepath.Join(u.HomeDir, ".ssh/config"), false},
		{"~/", u.HomeDir, false},
		{"~", u.HomeDir, false},
		{"~otheruser/.ssh/config", "~otheruser/.ssh/config", false},
		{"~otheruser", "~otheruser", false},
		{"/absolute/path", "/absolute/path", false},
		{"relative/path", "relative/path", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got, err := ExpandTilde(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ExpandTilde(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDefaultKeyPaths(t *testing.T) {
	paths := defaultKeyPaths()
	if len(paths) == 0 {
		t.Fatal("defaultKeyPaths returned empty slice")
	}

	expectedNames := []string{"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa"}
	for i, name := range expectedNames {
		if i >= len(paths) {
			t.Errorf("missing key path for %s", name)
			continue
		}
		if !strings.HasSuffix(paths[i], name) {
			t.Errorf("paths[%d] = %q, want suffix %q", i, paths[i], name)
		}
		if !strings.Contains(paths[i], ".ssh") {
			t.Errorf("paths[%d] = %q, should contain .ssh", i, paths[i])
		}
	}
}

// generateTestKey はテスト用の ed25519 秘密鍵を PEM 形式で生成する。
func generateTestKey(t *testing.T) (unencrypted []byte, encrypted []byte) {
	t.Helper()
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	unencrypted = pem.EncodeToMemory(block)

	encBlock, err := ssh.MarshalPrivateKeyWithPassphrase(privKey, "", []byte("test-passphrase"))
	if err != nil {
		t.Fatalf("failed to marshal private key with passphrase: %v", err)
	}
	encrypted = pem.EncodeToMemory(encBlock)
	return
}

func TestBuildAuthMethods_WithNilCallback(t *testing.T) {
	// SSH_AUTH_SOCK を無効化してエージェント認証を除外
	t.Setenv("SSH_AUTH_SOCK", "")

	unencrypted, _ := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test")
	if err := os.WriteFile(keyPath, unencrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{
		Name:          "test-host",
		HostName:      "example.com",
		Port:          22,
		User:          "user",
		IdentityFiles: []string{keyPath},
	}

	methods, closer := buildAuthMethods(host, nil)
	if closer != nil {
		if err := closer.Close(); err != nil {
			t.Errorf("failed to close agent connection: %v", err)
		}
	}

	if len(methods) == 0 {
		t.Fatal("expected at least one auth method with valid key file")
	}
}

func TestBuildAuthMethods_PasswordAuth(t *testing.T) {
	// SSH_AUTH_SOCK を無効化してエージェント認証を除外
	t.Setenv("SSH_AUTH_SOCK", "")

	host := core.SSHHost{
		Name:     "test-host",
		HostName: "example.com",
		Port:     22,
		User:     "user",
	}

	cbCalled := false
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		cbCalled = true
		if req.Type != core.CredentialPassword {
			t.Errorf("expected type %s, got %s", core.CredentialPassword, req.Type)
		}
		return core.CredentialResponse{Value: "secret"}, nil
	}

	methods, closer := buildAuthMethods(host, cb)
	if closer != nil {
		if err := closer.Close(); err != nil {
			t.Errorf("failed to close agent connection: %v", err)
		}
	}

	// コールバック付きなのでパスワード認証と keyboard-interactive が含まれるはず
	if len(methods) < 2 {
		t.Fatalf("expected at least 2 auth methods (password + keyboard-interactive), got %d", len(methods))
	}

	// パスワードコールバックが機能するか検証するために、
	// methods の中の PasswordCallback 型を探す
	// Note: ssh.AuthMethod はインターフェースなので直接型アサーションでは検証できないが、
	// コールバックが呼ばれたかで検証する
	if cbCalled {
		t.Error("callback should not be called during buildAuthMethods (lazy evaluation)")
	}
}

func TestBuildAuthMethods_KeyboardInteractive(t *testing.T) {
	// SSH_AUTH_SOCK を無効化してエージェント認証を除外
	t.Setenv("SSH_AUTH_SOCK", "")

	host := core.SSHHost{
		Name:     "test-host",
		HostName: "example.com",
		Port:     22,
		User:     "user",
	}

	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		if req.Type != core.CredentialKeyboardInteractive {
			return core.CredentialResponse{}, fmt.Errorf("unexpected type: %s", req.Type)
		}
		return core.CredentialResponse{Answers: []string{"answer1", "answer2"}}, nil
	}

	methods, closer := buildAuthMethods(host, cb)
	if closer != nil {
		if err := closer.Close(); err != nil {
			t.Errorf("failed to close agent connection: %v", err)
		}
	}

	// コールバック付きなので少なくともパスワード認証と keyboard-interactive が含まれる
	if len(methods) < 2 {
		t.Fatalf("expected at least 2 auth methods, got %d", len(methods))
	}
}

func TestBuildAuthMethods_NilCallbackNoPasswordOrKBI(t *testing.T) {
	// SSH_AUTH_SOCK を無効化してエージェント認証を除外
	t.Setenv("SSH_AUTH_SOCK", "")

	host := core.SSHHost{
		Name:     "test-host",
		HostName: "example.com",
		Port:     22,
		User:     "user",
	}

	methods, closer := buildAuthMethods(host, nil)
	if closer != nil {
		if err := closer.Close(); err != nil {
			t.Errorf("failed to close agent connection: %v", err)
		}
	}

	// nil コールバックの場合、パスワード認証・keyboard-interactive は追加されない
	// デフォルト鍵パスも存在しない場合、メソッドは 0 個のはず
	// （テスト環境にはデフォルト鍵が存在しない前提）
	for _, m := range methods {
		methodStr := fmt.Sprintf("%T", m)
		if strings.Contains(methodStr, "password") || strings.Contains(methodStr, "keyboard") {
			t.Errorf("nil callback should not include password or keyboard-interactive auth, got %s", methodStr)
		}
	}
}

func TestBuildAuthMethods_WithCallbackAndKeyFile(t *testing.T) {
	// SSH_AUTH_SOCK を無効化してエージェント認証を除外
	t.Setenv("SSH_AUTH_SOCK", "")

	unencrypted, _ := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test")
	if err := os.WriteFile(keyPath, unencrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{
		Name:          "test-host",
		HostName:      "example.com",
		Port:          22,
		User:          "user",
		IdentityFiles: []string{keyPath},
	}

	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Value: "secret"}, nil
	}

	methods, closer := buildAuthMethods(host, cb)
	if closer != nil {
		if err := closer.Close(); err != nil {
			t.Errorf("failed to close agent connection: %v", err)
		}
	}

	// 鍵ファイル + パスワード + keyboard-interactive の少なくとも 3 つ
	if len(methods) < 3 {
		t.Fatalf("expected at least 3 auth methods (key + password + keyboard-interactive), got %d", len(methods))
	}
}

func TestBuildAuthMethods_MultipleIdentityFiles(t *testing.T) {
	// SSH_AUTH_SOCK を無効化してエージェント認証を除外
	t.Setenv("SSH_AUTH_SOCK", "")

	key1, _ := generateTestKey(t)
	key2, _ := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath1 := filepath.Join(tmpDir, "id_test1")
	keyPath2 := filepath.Join(tmpDir, "id_test2")
	if err := os.WriteFile(keyPath1, key1, 0600); err != nil {
		t.Fatalf("failed to write key file 1: %v", err)
	}
	if err := os.WriteFile(keyPath2, key2, 0600); err != nil {
		t.Fatalf("failed to write key file 2: %v", err)
	}

	host := core.SSHHost{
		Name:          "test-host",
		HostName:      "example.com",
		Port:          22,
		User:          "user",
		IdentityFiles: []string{keyPath1, keyPath2},
	}

	methods, closer := buildAuthMethods(host, nil)
	if closer != nil {
		if err := closer.Close(); err != nil {
			t.Errorf("failed to close agent connection: %v", err)
		}
	}

	// 2 つの鍵ファイルから少なくとも 2 つの認証メソッドが生成されるはず
	if len(methods) < 2 {
		t.Fatalf("expected at least 2 auth methods for 2 identity files, got %d", len(methods))
	}
}
