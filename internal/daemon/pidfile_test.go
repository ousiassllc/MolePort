package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPIDFile_AcquireRelease_Lifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// ファイルが存在することを確認
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("PID file should exist after Acquire")
	}

	if err := pf.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	// ファイルが削除されたことを確認
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("PID file should not exist after Release")
	}
}

func TestPIDFile_DoubleAcquire_Fails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	pf1 := NewPIDFile(path)
	if err := pf1.Acquire(); err != nil {
		t.Fatalf("First Acquire: %v", err)
	}
	defer pf1.Release()

	pf2 := NewPIDFile(path)
	if err := pf2.Acquire(); err == nil {
		pf2.Release()
		t.Fatal("Second Acquire should fail due to flock contention")
	}
}

func TestPIDFile_Release_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if err := pf.Release(); err != nil {
		t.Fatalf("First Release: %v", err)
	}

	// 2回目の Release もエラーにならないこと
	if err := pf.Release(); err != nil {
		t.Fatalf("Second Release: %v", err)
	}
}

func TestPIDFile_AcquireAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	pf1 := NewPIDFile(path)
	if err := pf1.Acquire(); err != nil {
		t.Fatalf("First Acquire: %v", err)
	}
	if err := pf1.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	pf2 := NewPIDFile(path)
	if err := pf2.Acquire(); err != nil {
		t.Fatalf("Acquire after Release should succeed: %v", err)
	}
	defer pf2.Release()
}

func TestPIDFile_FilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer pf.Release()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestPIDFile_FileContent_MatchesPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	pf := NewPIDFile(path)

	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer pf.Release()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(content)
	if err != nil {
		t.Fatalf("PID file content is not a valid integer: %q", content)
	}

	if pid != os.Getpid() {
		t.Errorf("PID file content = %d, want %d", pid, os.Getpid())
	}
}

func TestIsRunning_NoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.pid")

	running, pid := IsRunning(path)
	if running {
		t.Error("IsRunning should return false for nonexistent file")
	}
	if pid != 0 {
		t.Errorf("PID = %d, want 0", pid)
	}
}

func TestIsRunning_CurrentPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.pid")
	currentPID := os.Getpid()

	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n", currentPID)), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, pid := IsRunning(path)
	if !running {
		t.Error("IsRunning should return true for current process PID")
	}
	if pid != currentPID {
		t.Errorf("PID = %d, want %d", pid, currentPID)
	}
}

func TestIsRunning_StalePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.pid")

	// 存在しないはずの大きな PID を書き込む
	if err := os.WriteFile(path, []byte("999999999\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, pid := IsRunning(path)
	if running {
		t.Errorf("IsRunning should return false for stale PID, got true with pid=%d", pid)
	}
	if pid != 0 {
		t.Errorf("PID = %d, want 0", pid)
	}
}

func TestIsRunning_InvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.pid")

	if err := os.WriteFile(path, []byte("not-a-number\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, pid := IsRunning(path)
	if running {
		t.Error("IsRunning should return false for invalid content")
	}
	if pid != 0 {
		t.Errorf("PID = %d, want 0", pid)
	}
}

func TestIsRunning_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.pid")

	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, pid := IsRunning(path)
	if running {
		t.Error("IsRunning should return false for empty file")
	}
	if pid != 0 {
		t.Errorf("PID = %d, want 0", pid)
	}
}

func TestIsRunning_NegativePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "negative.pid")

	if err := os.WriteFile(path, []byte("-1\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	running, pid := IsRunning(path)
	if running {
		t.Error("IsRunning should return false for negative PID")
	}
	if pid != 0 {
		t.Errorf("PID = %d, want 0", pid)
	}
}
