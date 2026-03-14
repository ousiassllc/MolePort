package client

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// SetCredentialHandler はクレデンシャル要求を処理するハンドラーを設定する。
// いつでも安全に呼び出せる。
func (c *IPCClient) SetCredentialHandler(handler CredentialHandler) {
	c.credMu.Lock()
	c.credHandler = handler
	c.credMu.Unlock()
}

// CredentialHandler は現在設定されているクレデンシャルハンドラーを返す。
func (c *IPCClient) CredentialHandler() CredentialHandler {
	c.credMu.RLock()
	h := c.credHandler
	c.credMu.RUnlock()
	return h
}

// handleCredentialRequest は credential.request 通知を処理し、credential.response を送信する。
func (c *IPCClient) handleCredentialRequest(notif protocol.Notification) {
	var req protocol.CredentialRequestNotification
	if err := json.Unmarshal(notif.Params, &req); err != nil {
		return
	}
	handler := c.CredentialHandler()
	if handler == nil {
		c.sendCredentialCancel(req.RequestID)
		return
	}
	resp, err := handler(req)
	if err != nil || resp == nil {
		c.sendCredentialCancel(req.RequestID)
		return
	}
	c.sendCredentialResult(resp)
}

// sendCredentialCancel はキャンセル応答を送信する。
func (c *IPCClient) sendCredentialCancel(requestID string) {
	ctx, cancel := context.WithTimeout(context.Background(), credentialResponseTimeout)
	defer cancel()
	params := protocol.CredentialResponseParams{
		RequestID: requestID,
		Cancelled: true,
	}
	var result protocol.CredentialResponseResult
	if err := c.Call(ctx, protocol.MethodCredentialResponse, params, &result); err != nil {
		slog.Warn("failed to send credential cancel", "request_id", requestID, "error", err)
	}
}

// sendCredentialResult はクレデンシャル応答を送信する。
func (c *IPCClient) sendCredentialResult(resp *protocol.CredentialResponseParams) {
	ctx, cancel := context.WithTimeout(context.Background(), credentialResponseTimeout)
	defer cancel()
	var result protocol.CredentialResponseResult
	if err := c.Call(ctx, protocol.MethodCredentialResponse, resp, &result); err != nil {
		slog.Warn("failed to send credential response", "request_id", resp.RequestID, "error", err)
	}
}
