package control

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// startTestDaemon は一時 configDir で実デーモンを起動し、テスト終了時に Stop する。
func startTestDaemon(t *testing.T, version string) (*daemon.Daemon, string) {
	t.Helper()
	dir := t.TempDir()
	sshConfigPath := filepath.Join(dir, "ssh_config")
	if err := os.WriteFile(sshConfigPath, []byte("Host testhost\n  HostName 127.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configYAML := "ssh_config_path: " + sshConfigPath + "\nsession:\n  auto_restore: false\nlog:\n  file: \"\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := daemon.New(dir, version)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() { _ = d.Stop() })
	return d, dir
}

func TestEnsureDaemon_Running(t *testing.T) {
	_, dir := startTestDaemon(t, "test")
	ctx := context.Background()

	client, err := EnsureDaemon(dir)
	if err != nil {
		t.Fatalf("EnsureDaemon() error: %v", err)
	}
	defer func() { _ = client.Close() }()

	if !client.IsConnected() {
		t.Error("client is not connected")
	}

	// daemon.status を呼び出して応答を確認
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var status protocol.DaemonStatusResult
	if err := client.Call(callCtx, "daemon.status", nil, &status); err != nil {
		t.Fatalf("daemon.status call error: %v", err)
	}

	if status.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", status.PID, os.Getpid())
	}
	if status.Version != "test" {
		t.Errorf("Version = %q, want %q", status.Version, "test")
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
	next, err := daemon.New(dir, "new")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := next.Start(context.Background()); err != nil {
		t.Fatalf("Start() after takeover error: %v", err)
	}
	_ = next.Stop()
}
