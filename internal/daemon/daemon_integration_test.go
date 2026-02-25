package daemon

import (
	"context"
	"os"
	"testing"
	"time"

	ipcclient "github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

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
	defer func() { _ = d.Stop() }()

	// IPC クライアントを接続
	client := ipcclient.NewIPCClient(SocketPath(dir))
	if err := client.Connect(); err != nil {
		t.Fatalf("client Connect() error: %v", err)
	}
	defer func() { _ = client.Close() }()

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
	defer func() { _ = d.Stop() }()

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
}
