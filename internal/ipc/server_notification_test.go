package ipc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ipcclient "github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestServerClient_Notification(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewIPCServer(sockPath, echoHandler)

	var connectedID string
	var mu sync.Mutex
	srv.OnClientConnected = func(clientID string) {
		mu.Lock()
		connectedID = clientID
		mu.Unlock()
	}

	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	client := connectTestClient(t, sockPath)

	// コールバックが呼ばれるまで待つ
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return connectedID != ""
	})

	mu.Lock()
	cid := connectedID
	mu.Unlock()

	notifParams, _ := json.Marshal(protocol.SSHEventNotification{
		Type: "connected",
		Host: "prod",
	})
	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  protocol.EventSSH,
		Params:  notifParams,
	}

	if err := srv.SendNotification(cid, notif); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}

	select {
	case got := <-client.Events():
		if got.Method != protocol.EventSSH {
			t.Errorf("notification method = %q, want %q", got.Method, protocol.EventSSH)
		}
		var evt protocol.SSHEventNotification
		if err := json.Unmarshal(got.Params, &evt); err != nil {
			t.Fatalf("unmarshal notification params: %v", err)
		}
		if evt.Host != "prod" {
			t.Errorf("event host = %q, want %q", evt.Host, "prod")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestServerClient_BroadcastNotification(t *testing.T) {
	srv, sockPath := startTestServer(t, echoHandler)

	client1 := connectTestClient(t, sockPath)
	client2 := connectTestClient(t, sockPath)

	waitFor(t, func() bool { return srv.ConnectedClients() == 2 })

	notifParams, _ := json.Marshal(map[string]string{"msg": "broadcast"})
	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  "event.broadcast",
		Params:  notifParams,
	}

	srv.BroadcastNotification(notif)

	for _, tc := range []struct {
		name   string
		client *ipcclient.IPCClient
	}{
		{"client1", client1},
		{"client2", client2},
	} {
		select {
		case got := <-tc.client.Events():
			if got.Method != "event.broadcast" {
				t.Errorf("%s: notification method = %q, want %q", tc.name, got.Method, "event.broadcast")
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: timed out waiting for broadcast notification", tc.name)
		}
	}
}

func TestServerClient_ClientDisconnect(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewIPCServer(sockPath, echoHandler)

	var disconnectedID atomic.Value
	srv.OnClientDisconnected = func(clientID string) {
		disconnectedID.Store(clientID)
	}

	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })

	client := connectTestClient(t, sockPath)
	waitFor(t, func() bool { return srv.ConnectedClients() == 1 })

	_ = client.Close()
	waitFor(t, func() bool { return srv.ConnectedClients() == 0 })

	// コールバックが呼ばれたことを確認
	waitFor(t, func() bool { return disconnectedID.Load() != nil })
}

func TestServerClient_ServerStop(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewIPCServer(sockPath, echoHandler)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start server: %v", err)
	}

	c := ipcclient.NewIPCClient(sockPath)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect client: %v", err)
	}

	// サーバーを停止する
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop server: %v", err)
	}

	// クライアントからの呼び出しはエラーになるべき
	err := c.Call(testCtxWithCleanup(t), "echo", nil, nil)
	if err == nil {
		t.Fatal("Call after server stop should return error")
	}

	_ = c.Close()
}

func TestServerClient_SendNotification_UnknownClient(t *testing.T) {
	srv, _ := startTestServer(t, echoHandler)

	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  "event.test",
	}

	err := srv.SendNotification("nonexistent", notif)
	if err == nil {
		t.Fatal("SendNotification to unknown client should return error")
	}
}

func TestIPCClient_CallNotConnected(t *testing.T) {
	c := ipcclient.NewIPCClient("/nonexistent.sock")
	err := c.Call(testCtxWithCleanup(t), "echo", nil, nil)
	if err == nil {
		t.Fatal("Call on unconnected client should return error")
	}
}

func TestIPCClient_CallContextTimeout(t *testing.T) {
	// レスポンスを返さないハンドラでタイムアウトを検証する
	slowHandler := func(_ string, method string, params json.RawMessage) (any, *protocol.RPCError) {
		time.Sleep(5 * time.Second)
		return nil, nil
	}
	_, sockPath := startTestServer(t, slowHandler)
	client := connectTestClient(t, sockPath)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.Call(ctx, "slow", nil, nil)
	if err == nil {
		t.Fatal("Call should return error on context timeout")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("error = %v, want %v", err, context.DeadlineExceeded)
	}
}
