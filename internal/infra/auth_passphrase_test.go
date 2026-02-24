package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestTryKeyFileWithPassphrase_Unencrypted(t *testing.T) {
	unencrypted, _ := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test")
	if err := os.WriteFile(keyPath, unencrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{Name: "test-host"}
	callbackCalled := false
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		callbackCalled = true
		return core.CredentialResponse{}, nil
	}

	auth, err := tryKeyFileWithPassphrase(keyPath, cb, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil auth method")
	}
	if callbackCalled {
		t.Error("callback should not be called for unencrypted key")
	}
}

func TestTryKeyFileWithPassphrase_Encrypted(t *testing.T) {
	_, encrypted := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test_enc")
	if err := os.WriteFile(keyPath, encrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{Name: "test-host"}
	callbackCalled := false
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		callbackCalled = true
		if req.Type != core.CredentialPassphrase {
			t.Errorf("expected type %s, got %s", core.CredentialPassphrase, req.Type)
		}
		if req.Host != "test-host" {
			t.Errorf("expected host 'test-host', got %q", req.Host)
		}
		if !strings.Contains(req.Prompt, keyPath) {
			t.Errorf("prompt should contain key path, got %q", req.Prompt)
		}
		return core.CredentialResponse{Value: "test-passphrase"}, nil
	}

	auth, err := tryKeyFileWithPassphrase(keyPath, cb, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil auth method")
	}
	if !callbackCalled {
		t.Error("callback should be called for encrypted key")
	}
}

func TestTryKeyFileWithPassphrase_EncryptedNilCallback(t *testing.T) {
	_, encrypted := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test_enc")
	if err := os.WriteFile(keyPath, encrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{Name: "test-host"}
	auth, err := tryKeyFileWithPassphrase(keyPath, nil, host)
	if err == nil {
		t.Fatal("expected error for encrypted key with nil callback")
	}
	if auth != nil {
		t.Error("expected nil auth method for encrypted key with nil callback")
	}
}

func TestTryKeyFileWithPassphrase_EncryptedCancelled(t *testing.T) {
	_, encrypted := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test_enc")
	if err := os.WriteFile(keyPath, encrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{Name: "test-host"}
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Cancelled: true}, nil
	}

	auth, err := tryKeyFileWithPassphrase(keyPath, cb, host)
	if err == nil {
		t.Fatal("expected error when passphrase input is cancelled")
	}
	if auth != nil {
		t.Error("expected nil auth method when passphrase input is cancelled")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error should mention 'cancelled', got %q", err.Error())
	}
}

func TestTryKeyFileWithPassphrase_EncryptedWrongPassphrase(t *testing.T) {
	_, encrypted := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test_enc")
	if err := os.WriteFile(keyPath, encrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{Name: "test-host"}
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Value: "wrong-passphrase"}, nil
	}

	auth, err := tryKeyFileWithPassphrase(keyPath, cb, host)
	if err == nil {
		t.Fatal("expected error for wrong passphrase")
	}
	if auth != nil {
		t.Error("expected nil auth method for wrong passphrase")
	}
}

func TestTryKeyFileWithPassphrase_CallbackError(t *testing.T) {
	_, encrypted := generateTestKey(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test_enc")
	if err := os.WriteFile(keyPath, encrypted, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	host := core.SSHHost{Name: "test-host"}
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{}, fmt.Errorf("connection lost")
	}

	auth, err := tryKeyFileWithPassphrase(keyPath, cb, host)
	if err == nil {
		t.Fatal("expected error when callback returns error")
	}
	if auth != nil {
		t.Error("expected nil auth method when callback returns error")
	}
	if !strings.Contains(err.Error(), "connection lost") {
		t.Errorf("error should contain callback error, got %q", err.Error())
	}
}
