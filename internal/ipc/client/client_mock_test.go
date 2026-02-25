package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"

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
