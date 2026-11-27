package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// newTestClient は net.Pipe() のクライアント側で IPCClient を初期化する。
// テスト用にソケット接続をバイパスする。
func newTestClient(t *testing.T, conn net.Conn) *IPCClient {
	t.Helper()
	c := &IPCClient{
		conn:    conn,
		enc:     json.NewEncoder(conn),
		scanner: bufio.NewScanner(conn),
		pending: make(map[int]chan *protocol.Response),
		eventCh: make(chan *protocol.Notification, 64),
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
	received []protocol.Request
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
func (s *mockServer) sendNotification(notif protocol.Notification) error {
	return s.enc.Encode(notif)
}

// readAndRespond はクライアントからのリクエストを1件読み込み、成功レスポンスを返す。
func (s *mockServer) readAndRespond() error {
	if !s.scanner.Scan() {
		return fmt.Errorf("scanner: %v", s.scanner.Err())
	}
	line := s.scanner.Bytes()

	var req protocol.Request
	if err := json.Unmarshal(line, &req); err != nil {
		return fmt.Errorf("unmarshal request: %w", err)
	}

	s.mu.Lock()
	s.received = append(s.received, req)
	s.mu.Unlock()

	result, _ := json.Marshal(protocol.CredentialResponseResult{OK: true})
	resp := protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
	return s.enc.Encode(resp)
}

// getReceived はロック付きで受信済みリクエストのコピーを返す。
func (s *mockServer) getReceived() []protocol.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]protocol.Request, len(s.received))
	copy(cp, s.received)
	return cp
}

// サーバーが応答前に接続を閉じたとき、保留中の Call は ctx の期限を待たずに
// 接続切断エラーで戻る。以後の Call も即座に失敗する。
func TestIPCClient_Call_ReleasedWhenConnectionClosed(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	c := newTestClient(t, clientConn)
	srv := newMockServer(t, serverConn)

	go func() {
		// リクエストを 1 件読んだら応答せずに接続を閉じる
		srv.scanner.Scan()
		_ = serverConn.Close()
	}()

	// 期限はテストが固まらないための上限であり、通常は使われない
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := c.Call(ctx, "daemon.status", nil, nil)
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Call error = %v, want connection closed error", err)
	}
	if err.Error() != "connection closed" {
		t.Errorf("Call error = %q, want %q", err.Error(), "connection closed")
	}

	<-c.done
	if err := c.Call(ctx, "daemon.status", nil, nil); err == nil {
		t.Fatal("Call after disconnect should fail")
	}
}
