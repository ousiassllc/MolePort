package daemon

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	ipcclient "github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// startTestDaemon は一時 configDir でデーモンを起動し、テスト終了時に Stop する。
func startTestDaemon(t *testing.T, version string) (*Daemon, string) {
	t.Helper()
	dir := createTestConfigDir(t)
	d, err := New(dir, version)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() { _ = d.Stop() })
	return d, dir
}

func assertDone(t *testing.T, d *Daemon, msg string) {
	t.Helper()
	select {
	case <-d.Done():
	case <-time.After(3 * time.Second):
		t.Fatal(msg)
	}
}

func TestDaemon_Services(t *testing.T) {
	d, err := New(createTestConfigDir(t), "test")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	s := d.Services()
	if s.Config != d.cfgMgr || s.SSH != d.sshMgr || s.Forward != d.fwdMgr || s.Version != d.versionChecker {
		t.Errorf("Services() = %+v, want references held by the daemon", s)
	}
	if s.Config == nil || s.SSH == nil || s.Forward == nil || s.Version == nil {
		t.Errorf("Services() has nil member: %+v", s)
	}
}

func TestDaemon_Info(t *testing.T) {
	before := time.Now()
	d, _ := startTestDaemon(t, "1.2.3")

	info := d.Info()
	if info.Version != "1.2.3" || info.PID != os.Getpid() {
		t.Errorf("Info() = %+v, want version 1.2.3 and pid %d", info, os.Getpid())
	}
	if info.StartedAt.Before(before) || info.StartedAt.After(time.Now()) {
		t.Errorf("StartedAt = %v, want between %v and now", info.StartedAt, before)
	}
}

func TestDaemon_Done_ClosedByShutdown(t *testing.T) {
	d, _ := startTestDaemon(t, "test")
	select {
	case <-d.Done():
		t.Fatal("Done() should not be closed before shutdown")
	default:
	}

	if err := d.Shutdown(false); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}

	assertDone(t, d, "Done() was not closed by Shutdown()")
}

func TestDaemon_Done_ClosedByDaemonShutdownIPC(t *testing.T) {
	d, dir := startTestDaemon(t, "test")

	c := ipcclient.NewIPCClient(SocketPath(dir))
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result protocol.DaemonShutdownResult
	if err := c.Call(ctx, "daemon.shutdown", protocol.DaemonShutdownParams{}, &result); err != nil {
		t.Fatalf("daemon.shutdown error: %v", err)
	}

	assertDone(t, d, "Done() was not closed by daemon.shutdown")
}

func TestDaemon_Stop_ConcurrentIsIdempotent(t *testing.T) {
	d, dir := startTestDaemon(t, "test")

	// トレイ「終了」・daemon.shutdown 起因の停止・OS シグナルが競合する状況を再現する。
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range cap(errs) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- d.Stop()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("Stop() error: %v", err)
		}
	}
	assertDone(t, d, "Done() was not closed by Stop()")
	if _, err := os.Stat(PIDFilePath(dir)); !os.IsNotExist(err) {
		t.Errorf("PID file should be released after Stop(), stat err = %v", err)
	}
}
