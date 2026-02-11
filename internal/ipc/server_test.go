package ipc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testCtxWithCleanup(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// echoHandler はメソッド名に応じた固定レスポンスを返すテスト用ハンドラ。
func echoHandler(_ string, method string, params json.RawMessage) (any, *RPCError) {
	switch method {
	case "echo":
		return json.RawMessage(params), nil
	case "error":
		return nil, &RPCError{Code: InternalError, Message: "test error"}
	default:
		return nil, &RPCError{Code: MethodNotFound, Message: "method not found"}
	}
}

func startTestServer(t *testing.T, handler HandlerFunc) (*IPCServer, string) {
	t.Helper()
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewIPCServer(sockPath, handler)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })
	return srv, sockPath
}

func connectTestClient(t *testing.T, sockPath string) *IPCClient {
	t.Helper()
	client := NewIPCClient(sockPath)
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestServerClient_BasicCall(t *testing.T) {
	_, sockPath := startTestServer(t, echoHandler)
	client := connectTestClient(t, sockPath)

	params := map[string]string{"msg": "hello"}
	var result map[string]string
	if err := client.Call(testCtxWithCleanup(t), "echo", params, &result); err != nil {
		t.Fatalf("Call echo: %v", err)
	}

	if result["msg"] != "hello" {
		t.Errorf("result[msg] = %q, want %q", result["msg"], "hello")
	}
}

func TestServerClient_ErrorResponse(t *testing.T) {
	_, sockPath := startTestServer(t, echoHandler)
	client := connectTestClient(t, sockPath)

	err := client.Call(testCtxWithCleanup(t), "error", nil, nil)
	if err == nil {
		t.Fatal("Call should return error")
	}

	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("error should be *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != InternalError {
		t.Errorf("RPCError.Code = %d, want %d", rpcErr.Code, InternalError)
	}
	if rpcErr.Message != "test error" {
		t.Errorf("RPCError.Message = %q, want %q", rpcErr.Message, "test error")
	}
}

func TestServerClient_MultipleClients(t *testing.T) {
	_, sockPath := startTestServer(t, echoHandler)

	const numClients = 5
	var wg sync.WaitGroup

	for i := range numClients {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			client := connectTestClient(t, sockPath)

			params := map[string]int{"n": n}
			var result map[string]int
			if err := client.Call(testCtxWithCleanup(t), "echo", params, &result); err != nil {
				t.Errorf("client %d: Call echo: %v", n, err)
				return
			}
			if result["n"] != n {
				t.Errorf("client %d: result[n] = %d, want %d", n, result["n"], n)
			}
		}(i)
	}

	wg.Wait()
}

func TestServerClient_ConnectedClients(t *testing.T) {
	srv, sockPath := startTestServer(t, echoHandler)

	if srv.ConnectedClients() != 0 {
		t.Fatalf("ConnectedClients = %d, want 0", srv.ConnectedClients())
	}

	client1 := connectTestClient(t, sockPath)
	// クライアント接続の反映を待つ
	waitFor(t, func() bool { return srv.ConnectedClients() == 1 })

	client2 := connectTestClient(t, sockPath)
	waitFor(t, func() bool { return srv.ConnectedClients() == 2 })

	client1.Close()
	waitFor(t, func() bool { return srv.ConnectedClients() == 1 })

	client2.Close()
	waitFor(t, func() bool { return srv.ConnectedClients() == 0 })
}

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
	t.Cleanup(func() { srv.Stop() })

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

	notifParams, _ := json.Marshal(SSHEventNotification{
		Type: "connected",
		Host: "prod",
	})
	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  "event.ssh",
		Params:  notifParams,
	}

	if err := srv.SendNotification(cid, notif); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}

	select {
	case got := <-client.Events():
		if got.Method != "event.ssh" {
			t.Errorf("notification method = %q, want %q", got.Method, "event.ssh")
		}
		var evt SSHEventNotification
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
	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  "event.broadcast",
		Params:  notifParams,
	}

	srv.BroadcastNotification(notif)

	for _, tc := range []struct {
		name   string
		client *IPCClient
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
	t.Cleanup(func() { srv.Stop() })

	client := connectTestClient(t, sockPath)
	waitFor(t, func() bool { return srv.ConnectedClients() == 1 })

	client.Close()
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

	client := NewIPCClient(sockPath)
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect client: %v", err)
	}

	// サーバーを停止する
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop server: %v", err)
	}

	// クライアントからの呼び出しはエラーになるべき
	err := client.Call(testCtxWithCleanup(t), "echo", nil, nil)
	if err == nil {
		t.Fatal("Call after server stop should return error")
	}

	client.Close()
}

func TestServerClient_SendNotification_UnknownClient(t *testing.T) {
	srv, _ := startTestServer(t, echoHandler)

	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  "event.test",
	}

	err := srv.SendNotification("nonexistent", notif)
	if err == nil {
		t.Fatal("SendNotification to unknown client should return error")
	}
}

func TestIPCClient_CallNotConnected(t *testing.T) {
	client := NewIPCClient("/nonexistent.sock")
	err := client.Call(testCtxWithCleanup(t), "echo", nil, nil)
	if err == nil {
		t.Fatal("Call on unconnected client should return error")
	}
}

func TestIPCClient_CallContextTimeout(t *testing.T) {
	// レスポンスを返さないハンドラでタイムアウトを検証する
	slowHandler := func(_ string, method string, params json.RawMessage) (any, *RPCError) {
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

// waitFor は条件が満たされるまで最大 2 秒待つヘルパー。
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("waitFor: condition not met within timeout")
}
