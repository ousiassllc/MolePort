package ipc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ipcclient "github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func testCtxWithCleanup(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// echoHandler はメソッド名に応じた固定レスポンスを返すテスト用ハンドラ。
func echoHandler(_ string, method string, params json.RawMessage) (any, *protocol.RPCError) {
	switch method {
	case "echo":
		return json.RawMessage(params), nil
	case "error":
		return nil, &protocol.RPCError{Code: protocol.InternalError, Message: "test error"}
	default:
		return nil, &protocol.RPCError{Code: protocol.MethodNotFound, Message: "method not found"}
	}
}

func startTestServer(t *testing.T, handler HandlerFunc) (*IPCServer, string) {
	t.Helper()
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewIPCServer(sockPath, handler)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	return srv, sockPath
}

func connectTestClient(t *testing.T, sockPath string) *ipcclient.IPCClient {
	t.Helper()
	c := ipcclient.NewIPCClient(sockPath)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect client: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
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

	rpcErr, ok := err.(*protocol.RPCError)
	if !ok {
		t.Fatalf("error should be *protocol.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != protocol.InternalError {
		t.Errorf("RPCError.Code = %d, want %d", rpcErr.Code, protocol.InternalError)
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

	_ = client1.Close()
	waitFor(t, func() bool { return srv.ConnectedClients() == 1 })

	_ = client2.Close()
	waitFor(t, func() bool { return srv.ConnectedClients() == 0 })
}

// TestServerClient_ConcurrentCredentialExchange は、readLoop が同期的にハンドラーを
// 呼び出している場合に発生するデッドロックを検証する回帰テスト（Issue #81）。
//
// シナリオ:
//  1. クライアントが forward.start RPC を送信
//  2. サーバーハンドラーが credential.request 通知を返送し、credential.response を待機
//  3. クライアントが通知を受信し credential.response RPC を送信
//  4. readLoop が同期的だと手順3のメッセージを読めずデッドロックする
func TestServerClient_ConcurrentCredentialExchange(t *testing.T) {
	// credential.response 用のチャネル（ハンドラー間でデータを受け渡す）
	credCh := make(chan string, 1)

	var srv *IPCServer

	handler := func(clientID string, method string, params json.RawMessage) (any, *protocol.RPCError) {
		switch method {
		case "forward.start":
			// credential.request 通知をクライアントに送信
			notif := protocol.Notification{
				JSONRPC: protocol.JSONRPCVersion,
				Method:  "credential.request",
				Params:  json.RawMessage(`{"request_id":"cr-1","type":"password","host":"test","prompt":"Password:"}`),
			}
			if err := srv.SendNotification(clientID, notif); err != nil {
				return nil, &protocol.RPCError{Code: protocol.InternalError, Message: err.Error()}
			}
			// credential.response が来るまで待機（タイムアウト付き）
			select {
			case val := <-credCh:
				return map[string]string{"password": val}, nil
			case <-time.After(3 * time.Second):
				return nil, &protocol.RPCError{Code: protocol.InternalError, Message: "credential timeout"}
			}

		case "credential.response":
			var p struct {
				RequestID string `json:"request_id"`
				Value     string `json:"value"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, &protocol.RPCError{Code: protocol.InvalidParams, Message: err.Error()}
			}
			credCh <- p.Value
			return map[string]bool{"ok": true}, nil

		default:
			return nil, &protocol.RPCError{Code: protocol.MethodNotFound, Message: "not found"}
		}
	}

	var sockPath string
	srv, sockPath = startTestServer(t, handler)
	client := connectTestClient(t, sockPath)
	waitFor(t, func() bool { return srv.ConnectedClients() == 1 })

	// クライアント側: credential.request 通知を受けたら credential.response を返す
	client.SetCredentialHandler(func(req protocol.CredentialRequestNotification) (*protocol.CredentialResponseParams, error) {
		return &protocol.CredentialResponseParams{
			RequestID: req.RequestID,
			Value:     "my-secret",
		}, nil
	})

	// forward.start を呼び出し — デッドロックしなければ成功する
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result map[string]string
	if err := client.Call(ctx, "forward.start", map[string]string{"name": "test"}, &result); err != nil {
		t.Fatalf("forward.start should succeed but got: %v", err)
	}
	if result["password"] != "my-secret" {
		t.Errorf("password = %q, want %q", result["password"], "my-secret")
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
