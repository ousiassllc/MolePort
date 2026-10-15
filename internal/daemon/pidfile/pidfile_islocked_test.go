package pidfile

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestIsLocked_NoFile(t *testing.T) {
	locked, pid := IsLocked(filepath.Join(t.TempDir(), "missing.pid"))
	if locked || pid != 0 {
		t.Errorf("IsLocked() = (%v, %d), want (false, 0)", locked, pid)
	}
}

func TestIsLocked_StalePIDFile(t *testing.T) {
	// 誰もロックしていない PID ファイル（PID 再利用で生存しているように見えても）は「ロックなし」。
	path := filepath.Join(t.TempDir(), "stale.pid")
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	locked, pid := IsLocked(path)
	if locked || pid != 0 {
		t.Errorf("IsLocked() = (%v, %d), want (false, 0)", locked, pid)
	}
	if running, _ := IsRunning(path); !running {
		t.Fatal("precondition: IsRunning should report the (re-used) PID as alive")
	}
}

func TestIsLocked_HeldByOtherDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "held.pid")
	pf := NewPIDFile(path)
	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = pf.Release() }()

	locked, pid := IsLocked(path)
	if !locked || pid != os.Getpid() {
		t.Errorf("IsLocked() = (%v, %d), want (true, %d)", locked, pid, os.Getpid())
	}
}

func TestIsLocked_DoesNotDisturbLockOrContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probe.pid")
	pf := NewPIDFile(path)
	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	for range 3 {
		if locked, _ := IsLocked(path); !locked {
			t.Fatal("IsLocked should keep reporting true while the lock is held")
		}
	}
	data, err := os.ReadFile(path) //nolint:gosec // path は t.TempDir() 配下のテスト用ファイル
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != strconv.Itoa(os.Getpid())+"\n" {
		t.Errorf("PID file content = %q, want unchanged", data)
	}

	if err := pf.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if locked, _ := IsLocked(path); locked {
		t.Error("IsLocked should be false after Release")
	}
}

func TestIsLocked_ProbeReleasesLock(t *testing.T) {
	// IsLocked が（ロックなしの場合に）ロックを取りっぱなしにしないこと。
	path := filepath.Join(t.TempDir(), "probe-release.pid")
	if err := os.WriteFile(path, []byte("1\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if locked, _ := IsLocked(path); locked {
		t.Fatal("IsLocked() = true for unlocked file")
	}

	pf := NewPIDFile(path)
	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire after IsLocked probe: %v", err)
	}
	_ = pf.Release()
}
