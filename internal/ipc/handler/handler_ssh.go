package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func (h *Handler) sshConnect(clientID string, params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.SSHConnectParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	// クレデンシャルコールバックを構築
	cb := h.buildCredentialCallback(clientID, p.Host)

	if err := h.sshMgr.ConnectWithCallback(p.Host, cb); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.SSHConnectResult{
		Host:   p.Host,
		Status: "connected",
	}, nil
}

// buildCredentialCallback はクライアントへの通知とレスポンス待機を行うコールバックを構築する。
func (h *Handler) buildCredentialCallback(clientID string, _ string) core.CredentialCallback {
	if h.sender == nil {
		return nil
	}
	return func(req core.CredentialRequest) (core.CredentialResponse, error) {
		reqID := fmt.Sprintf("cr-%d", h.credNextID.Add(1))

		// レスポンス待機用チャネルを登録
		ch := make(chan protocol.CredentialResponseParams, 1)
		h.credMu.Lock()
		h.credPending[reqID] = ch
		h.credMu.Unlock()

		defer func() {
			h.credMu.Lock()
			delete(h.credPending, reqID)
			h.credMu.Unlock()
		}()

		// credential.request 通知をクライアントに送信
		notif := protocol.CredentialRequestNotification{
			RequestID: reqID,
			Type:      string(req.Type),
			Host:      req.Host,
			Prompt:    req.Prompt,
		}
		if len(req.Prompts) > 0 {
			notif.Prompts = make([]protocol.PromptData, len(req.Prompts))
			for i, p := range req.Prompts {
				notif.Prompts[i] = protocol.PromptData{Prompt: p.Prompt, Echo: p.Echo}
			}
		}

		data, err := json.Marshal(notif)
		if err != nil {
			return core.CredentialResponse{}, fmt.Errorf("marshal credential request: %w", err)
		}

		if err := h.sender.SendNotification(clientID, protocol.Notification{
			JSONRPC: protocol.JSONRPCVersion,
			Method:  "credential.request",
			Params:  data,
		}); err != nil {
			return core.CredentialResponse{}, fmt.Errorf("send credential request: %w", err)
		}

		// レスポンスを待機（タイムアウト付き）
		select {
		case resp := <-ch:
			if resp.Cancelled {
				return core.CredentialResponse{}, fmt.Errorf("credential cancelled")
			}
			return core.CredentialResponse{
				RequestID: resp.RequestID,
				Value:     resp.Value,
				Answers:   resp.Answers,
			}, nil
		case <-time.After(credentialTimeout):
			return core.CredentialResponse{}, fmt.Errorf("credential timeout")
		}
	}
}

// credentialResponse はクライアントからのクレデンシャル応答を処理する。
func (h *Handler) credentialResponse(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.CredentialResponseParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	h.credMu.Lock()
	ch, ok := h.credPending[p.RequestID]
	h.credMu.Unlock()

	if !ok {
		return nil, &protocol.RPCError{Code: protocol.InvalidParams, Message: "no pending credential request for id: " + p.RequestID}
	}

	// 非ブロッキングで送信（チャネルはバッファ1）
	select {
	case ch <- p:
	default:
	}

	return protocol.CredentialResponseResult{OK: true}, nil
}

func (h *Handler) sshDisconnect(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.SSHDisconnectParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.sshMgr.Disconnect(p.Host); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.SSHDisconnectResult{
		Host:   p.Host,
		Status: "disconnected",
	}, nil
}
