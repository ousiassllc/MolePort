package handler

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// mockNotificationSender はテスト用の通知送信モック。
type mockNotificationSender struct {
	mu            sync.Mutex
	notifications []protocol.Notification
	clientID      string
}

func (m *mockNotificationSender) SendNotification(clientID string, notification protocol.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientID = clientID
	m.notifications = append(m.notifications, notification)
	return nil
}

func (m *mockNotificationSender) getNotifications() []protocol.Notification {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]protocol.Notification, len(m.notifications))
	copy(cp, m.notifications)
	return cp
}

func TestHandler_SSHConnect_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.SSHConnectParams{Host: "prod"})
	result, rpcErr := h.Handle("client-1", "ssh.connect", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	connectResult, ok := result.(protocol.SSHConnectResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.SSHConnectResult", result)
	}
	if connectResult.Host != "prod" {
		t.Errorf("host = %q, want %q", connectResult.Host, "prod")
	}
	if connectResult.Status != "connected" {
		t.Errorf("status = %q, want %q", connectResult.Status, "connected")
	}
}

func TestHandler_SSHConnect_Error(t *testing.T) {
	h, sshMgr, _, _ := newTestHandler()
	sshMgr.connectFn = func(hostName string) error {
		return fmt.Errorf("host %q not found", hostName)
	}

	params := mustMarshal(t, protocol.SSHConnectParams{Host: "nonexistent"})
	_, rpcErr := h.Handle("client-1", "ssh.connect", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != protocol.HostNotFound {
		t.Errorf("error code = %d, want %d (HostNotFound)", rpcErr.Code, protocol.HostNotFound)
	}
}

func TestHandler_SSHDisconnect_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.SSHDisconnectParams{Host: "prod"})
	result, rpcErr := h.Handle("client-1", "ssh.disconnect", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	discResult, ok := result.(protocol.SSHDisconnectResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.SSHDisconnectResult", result)
	}
	if discResult.Status != "disconnected" {
		t.Errorf("status = %q, want %q", discResult.Status, "disconnected")
	}
}

func TestHandler_SSHDisconnect_NotConnected(t *testing.T) {
	h, sshMgr, _, _ := newTestHandler()
	sshMgr.disconnFn = func(hostName string) error {
		return fmt.Errorf("host %q not connected", hostName)
	}

	params := mustMarshal(t, protocol.SSHDisconnectParams{Host: "prod"})
	_, rpcErr := h.Handle("client-1", "ssh.disconnect", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != protocol.NotConnected {
		t.Errorf("error code = %d, want %d (NotConnected)", rpcErr.Code, protocol.NotConnected)
	}
}

// --- クレデンシャル認証テスト ---

func TestHandler_CredentialResponse_NoPending(t *testing.T) {
	h, _, _, _ := newTestHandler()
	params, _ := json.Marshal(protocol.CredentialResponseParams{
		RequestID: "cr-nonexistent",
		Value:     "secret",
	})

	_, rpcErr := h.Handle("client-1", "credential.response", params)
	if rpcErr == nil {
		t.Fatal("expected error for non-existent credential request")
	} else if rpcErr.Code != protocol.InvalidParams {
		t.Errorf("expected InvalidParams error code, got %d", rpcErr.Code)
	}
}

func TestHandler_CredentialResponse_RoutesToPending(t *testing.T) {
	h, _, _, _ := newTestHandler()

	reqID := "cr-test-1"
	ch := make(chan protocol.CredentialResponseParams, 1)
	h.credMu.Lock()
	h.credPending[reqID] = ch
	h.credMu.Unlock()

	params, _ := json.Marshal(protocol.CredentialResponseParams{
		RequestID: reqID,
		Value:     "my-password",
	})

	result, rpcErr := h.Handle("client-1", "credential.response", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	credResult, ok := result.(protocol.CredentialResponseResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}
	if !credResult.OK {
		t.Error("expected OK=true")
	}

	// チャネルにレスポンスが送信されたことを確認
	select {
	case resp := <-ch:
		if resp.Value != "my-password" {
			t.Errorf("expected value 'my-password', got %q", resp.Value)
		}
		if resp.RequestID != reqID {
			t.Errorf("expected request_id %q, got %q", reqID, resp.RequestID)
		}
	default:
		t.Fatal("expected credential response in channel")
	}
}

func TestHandler_BuildCredentialCallback_SendsNotification(t *testing.T) {
	h, _, _, _ := newTestHandler()
	sender := &mockNotificationSender{}
	h.SetSender(sender)

	cb := h.buildCredentialCallback("client-1", "test-host")
	if cb == nil {
		t.Fatal("callback should not be nil when sender is set")
	}

	// コールバックを goroutine で実行し、レスポンスをシミュレート
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := cb(core.CredentialRequest{
			Type:   core.CredentialPassword,
			Host:   "test-host",
			Prompt: "Password:",
		})
		if err != nil {
			t.Errorf("unexpected callback error: %v", err)
			return
		}
		if resp.Value != "secret-pwd" {
			t.Errorf("expected value 'secret-pwd', got %q", resp.Value)
		}
	}()

	// 通知が送信されるまで待機
	time.Sleep(50 * time.Millisecond)

	notifications := sender.getNotifications()
	if len(notifications) == 0 {
		t.Fatal("expected credential.request notification to be sent")
	}

	notif := notifications[0]
	if notif.Method != "credential.request" {
		t.Errorf("expected method 'credential.request', got %q", notif.Method)
	}

	// 通知の内容を解析して request_id を取得
	var credReq protocol.CredentialRequestNotification
	if err := json.Unmarshal(notif.Params, &credReq); err != nil {
		t.Fatalf("failed to unmarshal notification params: %v", err)
	}
	if credReq.Type != "password" {
		t.Errorf("expected type 'password', got %q", credReq.Type)
	}
	if credReq.Host != "test-host" {
		t.Errorf("expected host 'test-host', got %q", credReq.Host)
	}

	// credential.response を送信
	respParams, _ := json.Marshal(protocol.CredentialResponseParams{
		RequestID: credReq.RequestID,
		Value:     "secret-pwd",
	})
	if _, err := h.Handle("client-1", "credential.response", respParams); err != nil {
		t.Fatalf("Handle credential.response failed: %v", err)
	}

	// コールバックの完了を待機
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for callback to complete")
	}
}

func TestHandler_BuildCredentialCallback_NilSender(t *testing.T) {
	h, _, _, _ := newTestHandler()
	// sender が nil の場合、コールバックは nil を返す
	cb := h.buildCredentialCallback("client-1", "test-host")
	if cb != nil {
		t.Error("callback should be nil when sender is nil")
	}
}
