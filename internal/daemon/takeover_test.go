package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/daemon/pidfile"
	"github.com/ousiassllc/moleport/internal/ipc"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// fakeHeadlessDaemon は headless デーモンの振る舞い（PID ロック保持 + daemon.shutdown 受信）を
// 同一プロセス内のゴルーチンで再現する。
type fakeHeadlessDaemon struct {
	pidFile *pidfile.PIDFile
	server  *ipc.IPCServer

	mu       sync.Mutex
	purge    *bool
	released chan struct{}
}

// startFakeHeadlessDaemon は PID ロックを保持して IPC サーバーを起動する。
// releaseOnShutdown が true なら daemon.shutdown を受けて shutdownDelay 後にロックを解放する。
func startFakeHeadlessDaemon(t *testing.T, dir string, releaseOnShutdown bool, shutdownDelay time.Duration) *fakeHeadlessDaemon {
	t.Helper()
	f := &fakeHeadlessDaemon{
		pidFile:  pidfile.NewPIDFile(PIDFilePath(dir)),
		released: make(chan struct{}),
	}
	if err := f.pidFile.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	f.server = ipc.NewIPCServer(SocketPath(dir), func(_ string, method string, params json.RawMessage) (any, *protocol.RPCError) {
		if method != "daemon.shutdown" {
			return nil, &protocol.RPCError{Code: protocol.MethodNotFound, Message: "unexpected " + method}
		}
		var p protocol.DaemonShutdownParams
		if len(params) > 0 {
			_ = json.Unmarshal(params, &p)
		}
		f.mu.Lock()
		f.purge = &p.Purge
		f.mu.Unlock()

		if releaseOnShutdown {
			go func() {
				time.Sleep(shutdownDelay)
				_ = f.pidFile.Release()
				close(f.released)
			}()
		}
		return protocol.DaemonShutdownResult{OK: true}, nil
	})
	if err := f.server.Start(context.Background()); err != nil {
		_ = f.pidFile.Release()
		t.Fatalf("server.Start: %v", err)
	}

	t.Cleanup(func() {
		_ = f.server.Stop()
		_ = f.pidFile.Release()
	})
	return f
}

func (f *fakeHeadlessDaemon) receivedPurge() (purge, received bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.purge == nil {
		return false, false
	}
	return *f.purge, true
}

func TestTakeover_NoPIDFile(t *testing.T) {
	pid, err := Takeover(createTestConfigDir(t), time.Second)
	if pid != 0 || err != nil {
		t.Errorf("Takeover() = (%d, %v), want (0, nil)", pid, err)
	}
}

func TestTakeover_StalePIDFile(t *testing.T) {
	// PID が再利用されて生存して見える（自プロセスの PID）が、誰もロックしていない PID ファイル。
	dir := createTestConfigDir(t)
	if err := os.WriteFile(PIDFilePath(dir), []byte(strconv.Itoa(os.Getpid())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	pid, err := Takeover(dir, time.Second)
	if pid != 0 || err != nil {
		t.Errorf("Takeover() = (%d, %v), want (0, nil)", pid, err)
	}
	if _, statErr := os.Stat(PIDFilePath(dir)); statErr != nil {
		t.Errorf("stale PID file should be left for Acquire() to overwrite: %v", statErr)
	}
}

func TestTakeover_LockedButSocketUnresponsive(t *testing.T) {
	dir := createTestConfigDir(t)
	pf := pidfile.NewPIDFile(PIDFilePath(dir))
	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = pf.Release() }()

	pid, err := Takeover(dir, time.Second)
	if !errors.Is(err, ErrTakeoverUnresponsive) {
		t.Fatalf("Takeover() error = %v, want ErrTakeoverUnresponsive", err)
	}
	if pid != os.Getpid() {
		t.Errorf("Takeover() pid = %d, want %d", pid, os.Getpid())
	}
	// 強制終了・PID ファイル削除をしていないこと。
	if locked, _ := pidfile.IsLocked(PIDFilePath(dir)); !locked {
		t.Error("lock should still be held (takeover must not kill or remove the PID file)")
	}
}

func TestTakeover_Success(t *testing.T) {
	dir := createTestConfigDir(t)
	fake := startFakeHeadlessDaemon(t, dir, true, 250*time.Millisecond)

	start := time.Now()
	pid, err := Takeover(dir, 5*time.Second)
	if err != nil {
		t.Fatalf("Takeover() error = %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("Takeover() pid = %d, want %d", pid, os.Getpid())
	}

	select {
	case <-fake.released:
	default:
		t.Error("Takeover returned before the lock was released")
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Errorf("Takeover returned after %v, want it to wait for the lock release", elapsed)
	}
	if purge, received := fake.receivedPurge(); !received || purge {
		t.Errorf("daemon.shutdown received=%v purge=%v, want received with purge=false", received, purge)
	}
	if locked, _ := pidfile.IsLocked(PIDFilePath(dir)); locked {
		t.Error("lock should be released after Takeover")
	}
}

func TestTakeover_Timeout(t *testing.T) {
	dir := createTestConfigDir(t)
	fake := startFakeHeadlessDaemon(t, dir, false, 0)

	pid, err := Takeover(dir, 300*time.Millisecond)
	if !errors.Is(err, ErrTakeoverTimeout) {
		t.Fatalf("Takeover() error = %v, want ErrTakeoverTimeout", err)
	}
	if pid != os.Getpid() {
		t.Errorf("Takeover() pid = %d, want %d", pid, os.Getpid())
	}
	if _, received := fake.receivedPurge(); !received {
		t.Error("daemon.shutdown should have been sent before waiting")
	}
	if locked, _ := pidfile.IsLocked(PIDFilePath(dir)); !locked {
		t.Error("lock should still be held after timeout (no forced kill)")
	}
}

func TestTakeover_RealDaemon(t *testing.T) {
	old, dir := startTestDaemon(t, "old")
	// headless デーモンの Wait() 相当: Done() を待って Stop() する。
	stopped := make(chan struct{})
	go func() {
		<-old.Done()
		_ = old.Stop()
		close(stopped)
	}()

	if _, err := Takeover(dir, TakeoverTimeout); err != nil {
		t.Fatalf("Takeover() error = %v", err)
	}
	<-stopped

	// 引き継ぎ後は新しいデーモンが PID を取得できる。
	next, err := New(dir, "new")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := next.Start(context.Background()); err != nil {
		t.Fatalf("Start() after takeover error: %v", err)
	}
	_ = next.Stop()
}
