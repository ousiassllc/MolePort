package daemon

import (
	"context"
	"testing"
	"time"

	ipcclient "github.com/ousiassllc/moleport/internal/ipc/client"
)

func TestDaemon_EventRouting(t *testing.T) {
	dir := createTestConfigDir(t)

	d, err := New(dir, "test")
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
