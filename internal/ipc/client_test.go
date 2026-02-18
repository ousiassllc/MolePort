package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// newTestClient は net.Pipe() のクライアント側で IPCClient を初期化する。
// テスト用にソケット接続をバイパスする。
func newTestClient(t *testing.T, conn net.Conn) *IPCClient {
	t.Helper()
	c := &IPCClient{
		conn:    conn,
		enc:     json.NewEncoder(conn),
		scanner: bufio.NewScanner(conn),
		pending: make(map[int]chan *Response),
		eventCh: make(chan *Notification, 64),
		done:    make(chan struct{}),
	}
	c.scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	c.connected.Store(true)
	go c.readLoop()
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// mockServer は net.Pipe() のサーバー側を処理する。
// クライアントからの credential.response RPC を受信して記録する。
type mockServer struct {
	conn    net.Conn
	scanner *bufio.Scanner
	enc     *json.Encoder

	mu       sync.Mutex
	received []Request
}

func newMockServer(t *testing.T, conn net.Conn) *mockServer {
	t.Helper()
	s := &mockServer{
		conn:    conn,
		scanner: bufio.NewScanner(conn),
		enc:     json.NewEncoder(conn),
	}
	s.scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return s
}

// sendNotification はクライアントに通知を送信する。
func (s *mockServer) sendNotification(notif Notification) error {
	return s.enc.Encode(notif)
}

// readAndRespond はクライアントからのリクエストを1件読み込み、成功レスポンスを返す。
func (s *mockServer) readAndRespond() error {
	if !s.scanner.Scan() {
		return fmt.Errorf("scanner: %v", s.scanner.Err())
	}
	line := s.scanner.Bytes()

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return fmt.Errorf("unmarshal request: %w", err)
	}

	s.mu.Lock()
	s.received = append(s.received, req)
	s.mu.Unlock()

	result, _ := json.Marshal(CredentialResponseResult{OK: true})
	resp := Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
	return s.enc.Encode(resp)
}

// getReceived はロック付きで受信済みリクエストのコピーを返す。
func (s *mockServer) getReceived() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]Request, len(s.received))
	copy(cp, s.received)
	return cp
}

func TestIPCClient_CredentialHandler_Success(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()

	server := newMockServer(t, serverConn)

	// ハンドラーがパスワードを返す
	client := newTestClient(t, clientConn)
	client.SetCredentialHandler(func(req CredentialRequestNotification) (*CredentialResponseParams, error) {
		return &CredentialResponseParams{
			RequestID: req.RequestID,
			Value:     "my-secret-password",
		}, nil
	})

	// サーバーから credential.request 通知を送信
	params, _ := json.Marshal(CredentialRequestNotification{
		RequestID: "req-123",
		Type:      "password",
		Host:      "prod-server",
		Prompt:    "Password:",
	})
	notif := Notification{
		JSONRPC: JSONRPCVersion,
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

	var credResp CredentialResponseParams
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
	client.SetCredentialHandler(func(req CredentialRequestNotification) (*CredentialResponseParams, error) {
		return nil, nil
	})

	params, _ := json.Marshal(CredentialRequestNotification{
		RequestID: "req-456",
		Type:      "password",
		Host:      "prod-server",
		Prompt:    "Password:",
	})
	notif := Notification{
		JSONRPC: JSONRPCVersion,
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

	var credResp CredentialResponseParams
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

	params, _ := json.Marshal(CredentialRequestNotification{
		RequestID: "req-789",
		Type:      "passphrase",
		Host:      "staging-server",
		Prompt:    "Enter passphrase for key:",
	})
	notif := Notification{
		JSONRPC: JSONRPCVersion,
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

	var credResp CredentialResponseParams
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
	client.SetCredentialHandler(func(req CredentialRequestNotification) (*CredentialResponseParams, error) {
		return &CredentialResponseParams{
			RequestID: req.RequestID,
			Value:     "pass",
		}, nil
	})

	// credential.request 通知を送信
	params, _ := json.Marshal(CredentialRequestNotification{
		RequestID: "req-event-test",
		Type:      "password",
		Host:      "test-server",
	})
	notif := Notification{
		JSONRPC: JSONRPCVersion,
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
