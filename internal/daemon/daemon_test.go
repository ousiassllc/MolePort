package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// createTestConfigDir はテスト用の設定ディレクトリを作成し、最小限の SSH config を配置する。
func createTestConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// 最小限の SSH config を作成
	sshConfigPath := filepath.Join(dir, "ssh_config")
	sshConfig := "Host testhost\n  HostName 127.0.0.1\n  Port 22\n  User testuser\n"
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0600); err != nil {
		t.Fatal(err)
	}

	// config.yaml を作成（SSH config パスをテスト用に設定）
	configYAML := "ssh_config_path: " + sshConfigPath + "\n" +
		"reconnect:\n" +
		"  enabled: false\n" +
		"  max_retries: 0\n" +
		"  initial_delay: 1s\n" +
		"  max_delay: 10s\n" +
		"session:\n" +
		"  auto_restore: false\n" +
		"log:\n" +
		"  level: debug\n" +
		"  file: \"\"\n"
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestDaemon_New(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if d.configDir != dir {
		t.Errorf("configDir = %q, want %q", d.configDir, dir)
	}
	if d.cfgMgr == nil {
		t.Error("cfgMgr is nil")
	}
	if d.sshMgr == nil {
		t.Error("sshMgr is nil")
	}
	if d.fwdMgr == nil {
		t.Error("fwdMgr is nil")
	}
	if d.broker == nil {
		t.Error("broker is nil")
	}
	if d.handler == nil {
		t.Error("handler is nil")
	}
	if d.server == nil {
		t.Error("server is nil")
	}
	if d.pidFile == nil {
		t.Error("pidFile is nil")
	}
}

func TestDaemon_StartStop(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// PID ファイルが存在することを確認
	pidPath := PIDFilePath(dir)
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file does not exist after Start")
	}

	// ソケットファイルが存在することを確認
	sockPath := SocketPath(dir)
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Error("socket file does not exist after Start")
	}

	// Stop
	if err := d.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	// ソケットファイルが削除されていることを確認
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file still exists after Stop")
	}
}

func TestDaemon_Status(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer d.Stop()

	status := d.Status()

	if status.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", status.PID, os.Getpid())
	}
	if status.StartedAt == "" {
		t.Error("StartedAt is empty")
	}
	if status.Uptime == "" {
		t.Error("Uptime is empty")
	}
	if status.ConnectedClients != 0 {
		t.Errorf("ConnectedClients = %d, want 0", status.ConnectedClients)
	}
	if status.ActiveSSHConnections != 0 {
		t.Errorf("ActiveSSHConnections = %d, want 0", status.ActiveSSHConnections)
	}
	if status.ActiveForwards != 0 {
		t.Errorf("ActiveForwards = %d, want 0", status.ActiveForwards)
	}
}

func TestDaemon_Shutdown(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Shutdown はコンテキストをキャンセルする
	if err := d.Shutdown(); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}

	// コンテキストがキャンセルされたことを確認
	select {
	case <-d.ctx.Done():
		// 期待通り
	case <-time.After(time.Second):
		t.Error("context was not cancelled after Shutdown")
	}

	// Stop で後処理
	d.Stop()
}

func TestDaemon_EventRouting(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer d.Stop()

	// IPC クライアントを接続
	client := ipc.NewIPCClient(SocketPath(dir))
	if err := client.Connect(); err != nil {
		t.Fatalf("client Connect() error: %v", err)
	}
	defer client.Close()

	// イベント購読
	callCtx, callCancel := context.WithTimeout(ctx, 5*time.Second)
	defer callCancel()

	subID, err := client.Subscribe(callCtx, []string{"ssh"})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}
	if subID == "" {
		t.Error("subscription ID is empty")
	}

	// 購読解除
	if err := client.Unsubscribe(callCtx, subID); err != nil {
		t.Fatalf("Unsubscribe() error: %v", err)
	}
}

func TestEnsureDaemon_NotRunning(t *testing.T) {
	dir := t.TempDir()

	_, err := EnsureDaemon(dir)
	if err == nil {
		t.Fatal("EnsureDaemon() should return error when daemon is not running")
	}
}

func TestEnsureDaemon_Running(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer d.Stop()

	client, err := EnsureDaemon(dir)
	if err != nil {
		t.Fatalf("EnsureDaemon() error: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("client is not connected")
	}

	// daemon.status を呼び出して応答を確認
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var status ipc.DaemonStatusResult
	if err := client.Call(callCtx, "daemon.status", nil, &status); err != nil {
		t.Fatalf("daemon.status call error: %v", err)
	}

	if status.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", status.PID, os.Getpid())
	}
}

func TestDaemon_DoubleStop(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if err := d.Stop(); err != nil {
		t.Fatalf("first Stop() error: %v", err)
	}
	// 二重 Stop がパニックしないことを確認
	if err := d.Stop(); err != nil {
		t.Fatalf("second Stop() error: %v", err)
	}
}

func TestSocketPath(t *testing.T) {
	got := SocketPath("/tmp/test")
	want := "/tmp/test/moleport.sock"
	if got != want {
		t.Errorf("SocketPath() = %q, want %q", got, want)
	}
}

func TestPIDFilePath(t *testing.T) {
	got := PIDFilePath("/tmp/test")
	want := "/tmp/test/moleport.pid"
	if got != want {
		t.Errorf("PIDFilePath() = %q, want %q", got, want)
	}
}
