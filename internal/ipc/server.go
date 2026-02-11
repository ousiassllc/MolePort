package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

// HandlerFunc は RPC リクエストを処理するハンドラ関数の型。
type HandlerFunc func(method string, params json.RawMessage) (any, *RPCError)

// IPCServer は Unix ドメインソケット上で JSON-RPC 2.0 通信を行うサーバー。
type IPCServer struct {
	socketPath string
	listener   net.Listener
	handler    HandlerFunc
	clients    map[string]*clientConn
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	nextID     atomic.Int64

	// コールバック用ミューテックス
	cbMu sync.RWMutex
	// OnClientConnected はクライアント接続時に呼ばれるコールバック。
	// Start() の前後どちらでも設定可能。
	OnClientConnected func(clientID string)
	// OnClientDisconnected はクライアント切断時に呼ばれるコールバック。
	// Start() の前後どちらでも設定可能。
	OnClientDisconnected func(clientID string)
}

// clientConn は接続中のクライアントを表す。
type clientConn struct {
	id   string
	conn net.Conn
	enc  *json.Encoder
	mu   sync.Mutex
}

// NewIPCServer は新しい IPCServer を生成する。
func NewIPCServer(socketPath string, handler HandlerFunc) *IPCServer {
	return &IPCServer{
		socketPath: socketPath,
		handler:    handler,
		clients:    make(map[string]*clientConn),
	}
}

// Start はソケットを作成し、クライアント接続の受け付けを開始する。
func (s *IPCServer) Start(ctx context.Context) error {
	// 古いソケットファイルがあれば削除する
	if _, err := os.Stat(s.socketPath); err == nil {
		if err := os.Remove(s.socketPath); err != nil {
			return fmt.Errorf("remove stale socket: %w", err)
		}
	}

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen unix: %w", err)
	}

	if err := os.Chmod(s.socketPath, 0600); err != nil {
		ln.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}

	s.listener = ln
	s.ctx, s.cancel = context.WithCancel(ctx)

	go s.acceptLoop()

	return nil
}

// Stop はリスナーを閉じ、全クライアント接続を切断し、ソケットファイルを削除する。
func (s *IPCServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}

	var firstErr error
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			firstErr = err
		}
	}

	s.mu.Lock()
	for _, c := range s.clients {
		c.conn.Close()
	}
	s.clients = make(map[string]*clientConn)
	s.mu.Unlock()

	os.Remove(s.socketPath)
	return firstErr
}

// ConnectedClients は接続中のクライアント数を返す。
func (s *IPCServer) ConnectedClients() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// SendNotification は指定クライアントに通知を送信する。
func (s *IPCServer) SendNotification(clientID string, notification Notification) error {
	s.mu.RLock()
	c, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("client %s not found", clientID)
	}
	return c.send(notification)
}

// BroadcastNotification は全クライアントに通知を送信する。
func (s *IPCServer) BroadcastNotification(notification Notification) {
	s.mu.RLock()
	clients := make([]*clientConn, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.RUnlock()

	for _, c := range clients {
		// 個々の送信エラーは無視する（切断中のクライアントなど）
		c.send(notification)
	}
}

func (s *IPCServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// リスナーが閉じられた場合は終了
			select {
			case <-s.ctx.Done():
				return
			default:
				continue
			}
		}

		id := fmt.Sprintf("client-%d", s.nextID.Add(1))
		c := &clientConn{
			id:   id,
			conn: conn,
			enc:  json.NewEncoder(conn),
		}

		s.mu.Lock()
		s.clients[id] = c
		s.mu.Unlock()

		s.cbMu.RLock()
		cb := s.OnClientConnected
		s.cbMu.RUnlock()
		if cb != nil {
			cb(id)
		}

		go s.readLoop(c)
	}
}

func (s *IPCServer) readLoop(c *clientConn) {
	defer func() {
		c.conn.Close()

		s.mu.Lock()
		delete(s.clients, c.id)
		s.mu.Unlock()

		s.cbMu.RLock()
		dcb := s.OnClientDisconnected
		s.cbMu.RUnlock()
		if dcb != nil {
			dcb(c.id)
		}
	}()

	scanner := bufio.NewScanner(c.conn)
	// デフォルトの 64KB バッファで十分だが、大きなメッセージに備える
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			// パースエラー: ID が不明なので null で返す
			resp := NewErrorResponse(nil, ParseError, "parse error")
			if err := c.send(resp); err != nil {
				return
			}
			continue
		}

		if req.JSONRPC != JSONRPCVersion {
			if req.ID != nil {
				resp := NewErrorResponse(req.ID, InvalidRequest, "invalid jsonrpc version")
				if err := c.send(resp); err != nil {
					return
				}
			}
			continue
		}

		// ID が nil の場合は通知（レスポンス不要）
		if req.ID == nil {
			s.handler(req.Method, req.Params)
			continue
		}

		result, rpcErr := s.handler(req.Method, req.Params)
		if rpcErr != nil {
			resp := NewErrorResponse(req.ID, rpcErr.Code, rpcErr.Message)
			if err := c.send(resp); err != nil {
				return
			}
			continue
		}

		resp, err := NewResponse(req.ID, result)
		if err != nil {
			resp = NewErrorResponse(req.ID, InternalError, "marshal result: "+err.Error())
		}
		if err := c.send(resp); err != nil {
			return
		}
	}
}

func (c *clientConn) send(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.enc.Encode(v); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}
