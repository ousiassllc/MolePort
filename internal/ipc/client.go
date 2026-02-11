package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// CredentialHandler はクレデンシャル要求を処理するコールバック関数の型。
// CLI や TUI がそれぞれの方法で実装する。
// Connect() の前に SetCredentialHandler で設定すること。
type CredentialHandler func(req CredentialRequestNotification) (*CredentialResponseParams, error)

// IPCClient は Unix ドメインソケット経由でデーモンと通信するクライアント。
type IPCClient struct {
	socketPath  string
	conn        net.Conn
	enc         *json.Encoder
	scanner     *bufio.Scanner
	nextID      atomic.Int64
	mu          sync.Mutex
	pending     map[int]chan *Response
	pendingMu   sync.Mutex
	eventCh     chan *Notification
	done        chan struct{}
	connected   atomic.Bool
	credHandler CredentialHandler
}

// NewIPCClient は新しい IPCClient を生成する。
func NewIPCClient(socketPath string) *IPCClient {
	return &IPCClient{
		socketPath: socketPath,
		pending:    make(map[int]chan *Response),
		eventCh:    make(chan *Notification, 64),
		done:       make(chan struct{}),
	}
}

// Connect はデーモンの Unix ソケットに接続し、受信ループを開始する。
func (c *IPCClient) Connect() error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("dial unix: %w", err)
	}

	c.conn = conn
	c.enc = json.NewEncoder(conn)
	c.scanner = bufio.NewScanner(conn)
	c.scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	c.connected.Store(true)

	go c.readLoop()

	return nil
}

// Close は接続を閉じ、チャネルをクリーンアップする。
func (c *IPCClient) Close() error {
	if !c.connected.Load() {
		return nil
	}
	c.connected.Store(false)

	var err error
	if c.conn != nil {
		err = c.conn.Close()
	}

	// readLoop の終了を待つ（タイムアウト付き）
	select {
	case <-c.done:
	case <-time.After(3 * time.Second):
	}

	// 保留中のリクエストをすべてエラーで解決する
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	return err
}

// Call は RPC メソッドを呼び出し、結果を待つ。
// result には応答の result フィールドがアンマーシャルされる。
// サーバーが RPCError を返した場合、*RPCError が Go error として返される。
// ctx でタイムアウトやキャンセルを制御できる。
func (c *IPCClient) Call(ctx context.Context, method string, params any, result any) error {
	if !c.connected.Load() {
		return errors.New("not connected")
	}

	id := int(c.nextID.Add(1))

	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		rawParams = data
	}

	req := Request{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
		Method:  method,
		Params:  rawParams,
	}

	ch := make(chan *Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	c.mu.Lock()
	err := c.enc.Encode(req)
	c.mu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return fmt.Errorf("send request: %w", err)
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return errors.New("connection closed")
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return ctx.Err()
	}
}

// Subscribe はイベントサブスクリプションを登録する。
func (c *IPCClient) Subscribe(ctx context.Context, types []string) (string, error) {
	params := EventsSubscribeParams{Types: types}
	var result EventsSubscribeResult
	if err := c.Call(ctx, "events.subscribe", params, &result); err != nil {
		return "", err
	}
	return result.SubscriptionID, nil
}

// Unsubscribe はイベントサブスクリプションを解除する。
func (c *IPCClient) Unsubscribe(ctx context.Context, subscriptionID string) error {
	params := EventsUnsubscribeParams{SubscriptionID: subscriptionID}
	var result EventsUnsubscribeResult
	return c.Call(ctx, "events.unsubscribe", params, &result)
}

// Events はイベント通知チャネルを返す。
func (c *IPCClient) Events() <-chan *Notification {
	return c.eventCh
}

// IsConnected は接続状態を返す。
func (c *IPCClient) IsConnected() bool {
	return c.connected.Load()
}

// SetCredentialHandler はクレデンシャル要求を処理するハンドラーを設定する。
// Connect() の前に呼び出すこと。
func (c *IPCClient) SetCredentialHandler(handler CredentialHandler) {
	c.credHandler = handler
}

func (c *IPCClient) readLoop() {
	defer func() {
		c.connected.Store(false)
		close(c.done)
	}()

	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// "id" フィールドの有無で Response と Notification を判別する
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		if _, hasID := raw["id"]; hasID {
			var resp Response
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}
			if resp.ID != nil {
				c.pendingMu.Lock()
				ch, ok := c.pending[*resp.ID]
				if ok {
					delete(c.pending, *resp.ID)
				}
				c.pendingMu.Unlock()
				if ok {
					ch <- &resp
				}
			}
		} else {
			var notif Notification
			if err := json.Unmarshal(line, &notif); err != nil {
				continue
			}
			// credential.request は専用ハンドラーで処理
			if notif.Method == "credential.request" {
				go c.handleCredentialRequest(notif)
				continue
			}
			select {
			case c.eventCh <- &notif:
			default:
				// チャネルが満杯の場合は通知を破棄する
			}
		}
	}
}

// handleCredentialRequest は credential.request 通知を処理し、credential.response を送信する。
func (c *IPCClient) handleCredentialRequest(notif Notification) {
	handler := c.credHandler
	if handler == nil {
		// ハンドラー未設定の場合、パラメータからリクエスト ID を取得してキャンセルを送信
		var req CredentialRequestNotification
		if err := json.Unmarshal(notif.Params, &req); err != nil {
			return
		}
		c.sendCredentialCancel(req.RequestID)
		return
	}

	var req CredentialRequestNotification
	if err := json.Unmarshal(notif.Params, &req); err != nil {
		return
	}

	resp, err := handler(req)
	if err != nil || resp == nil {
		// エラー or nil の場合、キャンセルを送信
		c.sendCredentialCancel(req.RequestID)
		return
	}

	c.sendCredentialResult(resp)
}

// sendCredentialCancel はクレデンシャル要求をキャンセルする credential.response を送信する。
func (c *IPCClient) sendCredentialCancel(requestID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	params := CredentialResponseParams{
		RequestID: requestID,
		Cancelled: true,
	}
	var result CredentialResponseResult
	_ = c.Call(ctx, "credential.response", params, &result)
}

// sendCredentialResult はクレデンシャル応答を credential.response で送信する。
func (c *IPCClient) sendCredentialResult(resp *CredentialResponseParams) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result CredentialResponseResult
	_ = c.Call(ctx, "credential.response", resp, &result)
}
