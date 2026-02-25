package client

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestIPCClient_CredentialHandler_Success(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()

	server := newMockServer(t, serverConn)

	// ハンドラーがパスワードを返す
	client := newTestClient(t, clientConn)
	client.SetCredentialHandler(func(req protocol.CredentialRequestNotification) (*protocol.CredentialResponseParams, error) {
		return &protocol.CredentialResponseParams{
			RequestID: req.RequestID,
			Value:     "my-secret-password",
		}, nil
	})

	// サーバーから credential.request 通知を送信
	params, _ := json.Marshal(protocol.CredentialRequestNotification{
		RequestID: "req-123",
		Type:      "password",
		Host:      "prod-server",
		Prompt:    "Password:",
	})
	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  "credential.request",
		Params:  params,
	}
	if err := server.sendNotification(notif); err != nil {
		t.Fatalf("sendNotification: %v", err)
	}

	// サーバー側でクライアントからの credential.response を受信
	if err := server.readAndRespond(); err != nil {
		t.Fatalf("readAndRespond: %v", err)
	}

	received := server.getReceived()
	if len(received) != 1 {
		t.Fatalf("received %d requests, want 1", len(received))
	}

	req := received[0]
	if req.Method != "credential.response" {
		t.Errorf("method = %q, want %q", req.Method, "credential.response")
	}

	var credResp protocol.CredentialResponseParams
	if err := json.Unmarshal(req.Params, &credResp); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if credResp.RequestID != "req-123" {
		t.Errorf("request_id = %q, want %q", credResp.RequestID, "req-123")
	}
	if credResp.Value != "my-secret-password" {
		t.Errorf("value = %q, want %q", credResp.Value, "my-secret-password")
	}
	if credResp.Cancelled {
		t.Error("cancelled = true, want false")
	}
}

func TestIPCClient_CredentialHandler_Cancelled(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()

	server := newMockServer(t, serverConn)

	// ハンドラーが nil を返す（ユーザーがキャンセルした場合）
	client := newTestClient(t, clientConn)
	client.SetCredentialHandler(func(req protocol.CredentialRequestNotification) (*protocol.CredentialResponseParams, error) {
		return nil, nil
	})

	params, _ := json.Marshal(protocol.CredentialRequestNotification{
		RequestID: "req-456",
		Type:      "password",
		Host:      "prod-server",
		Prompt:    "Password:",
	})
	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  "credential.request",
		Params:  params,
	}
	if err := server.sendNotification(notif); err != nil {
		t.Fatalf("sendNotification: %v", err)
	}

	if err := server.readAndRespond(); err != nil {
		t.Fatalf("readAndRespond: %v", err)
	}

	received := server.getReceived()
	if len(received) != 1 {
		t.Fatalf("received %d requests, want 1", len(received))
	}

	req := received[0]
	if req.Method != "credential.response" {
		t.Errorf("method = %q, want %q", req.Method, "credential.response")
	}

	var credResp protocol.CredentialResponseParams
	if err := json.Unmarshal(req.Params, &credResp); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if credResp.RequestID != "req-456" {
		t.Errorf("request_id = %q, want %q", credResp.RequestID, "req-456")
	}
	if !credResp.Cancelled {
		t.Error("cancelled = false, want true")
	}
}

func TestIPCClient_CredentialHandler_NoHandler(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()

	server := newMockServer(t, serverConn)

	// ハンドラーを設定しない
	client := newTestClient(t, clientConn)
	_ = client // ハンドラー未設定

	params, _ := json.Marshal(protocol.CredentialRequestNotification{
		RequestID: "req-789",
		Type:      "passphrase",
		Host:      "staging-server",
		Prompt:    "Enter passphrase for key:",
	})
	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  "credential.request",
		Params:  params,
	}
	if err := server.sendNotification(notif); err != nil {
		t.Fatalf("sendNotification: %v", err)
	}

	if err := server.readAndRespond(); err != nil {
		t.Fatalf("readAndRespond: %v", err)
	}

	received := server.getReceived()
	if len(received) != 1 {
		t.Fatalf("received %d requests, want 1", len(received))
	}

	req := received[0]
	if req.Method != "credential.response" {
		t.Errorf("method = %q, want %q", req.Method, "credential.response")
	}

	var credResp protocol.CredentialResponseParams
	if err := json.Unmarshal(req.Params, &credResp); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if credResp.RequestID != "req-789" {
		t.Errorf("request_id = %q, want %q", credResp.RequestID, "req-789")
	}
	if !credResp.Cancelled {
		t.Error("cancelled = false, want true")
	}
}

func TestIPCClient_CredentialHandler_NotForwardedToEventCh(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()

	server := newMockServer(t, serverConn)

	client := newTestClient(t, clientConn)
	client.SetCredentialHandler(func(req protocol.CredentialRequestNotification) (*protocol.CredentialResponseParams, error) {
		return &protocol.CredentialResponseParams{
			RequestID: req.RequestID,
			Value:     "pass",
		}, nil
	})

	// credential.request 通知を送信
	params, _ := json.Marshal(protocol.CredentialRequestNotification{
		RequestID: "req-event-test",
		Type:      "password",
		Host:      "test-server",
	})
	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  "credential.request",
		Params:  params,
	}
	if err := server.sendNotification(notif); err != nil {
		t.Fatalf("sendNotification: %v", err)
	}

	// サーバー側で credential.response を処理
	if err := server.readAndRespond(); err != nil {
		t.Fatalf("readAndRespond: %v", err)
	}

	// credential.request は eventCh に送信されないことを確認
	select {
	case got := <-client.Events():
		t.Errorf("credential.request should not be forwarded to eventCh, got method=%q", got.Method)
	case <-time.After(100 * time.Millisecond):
		// 期待通り: eventCh にはメッセージがない
	}
}
