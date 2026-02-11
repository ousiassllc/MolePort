package ipc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// DaemonInfo はデーモンの状態情報とシャットダウンを提供するインターフェース。
type DaemonInfo interface {
	Status() DaemonStatusResult
	Shutdown(purge bool) error
}

// NotificationSender はクライアントに通知を送信するインターフェース。
type NotificationSender interface {
	SendNotification(clientID string, notification Notification) error
}

// Handler は JSON-RPC メソッドをコアマネージャーにルーティングする。
type Handler struct {
	sshMgr core.SSHManager
	fwdMgr core.ForwardManager
	cfgMgr core.ConfigManager
	broker *EventBroker
	daemon DaemonInfo
	sender NotificationSender

	credMu      sync.Mutex
	credPending map[string]chan CredentialResponseParams
	credNextID  atomic.Int64
}

// NewHandler は新しい Handler を生成する。
func NewHandler(
	sshMgr core.SSHManager,
	fwdMgr core.ForwardManager,
	cfgMgr core.ConfigManager,
	broker *EventBroker,
	daemon DaemonInfo,
) *Handler {
	return &Handler{
		sshMgr:      sshMgr,
		fwdMgr:      fwdMgr,
		cfgMgr:      cfgMgr,
		broker:      broker,
		daemon:      daemon,
		credPending: make(map[string]chan CredentialResponseParams),
	}
}

// SetSender は通知送信用のサーバー参照を設定する。
// IPCServer の生成後に呼び出す。
func (h *Handler) SetSender(sender NotificationSender) {
	h.sender = sender
}

// Handle は JSON-RPC メソッドをディスパッチする。HandlerFunc として使用する。
func (h *Handler) Handle(clientID string, method string, params json.RawMessage) (any, *RPCError) {
	switch method {
	case "host.list":
		return h.hostList()
	case "host.reload":
		return h.hostReload()
	case "ssh.connect":
		return h.sshConnect(clientID, params)
	case "ssh.disconnect":
		return h.sshDisconnect(params)
	case "credential.response":
		return h.credentialResponse(params)
	case "forward.list":
		return h.forwardList(params)
	case "forward.add":
		return h.forwardAdd(params)
	case "forward.delete":
		return h.forwardDelete(params)
	case "forward.start":
		return h.forwardStart(params)
	case "forward.stop":
		return h.forwardStop(params)
	case "forward.stopAll":
		return h.forwardStopAll()
	case "session.list":
		return h.sessionList()
	case "session.get":
		return h.sessionGet(params)
	case "config.get":
		return h.configGet()
	case "config.update":
		return h.configUpdate(params)
	case "daemon.status":
		return h.daemonStatus()
	case "daemon.shutdown":
		return h.daemonShutdown(params)
	case "events.subscribe":
		return h.eventsSubscribe(clientID, params)
	case "events.unsubscribe":
		return h.eventsUnsubscribe(params)
	default:
		return nil, &RPCError{Code: MethodNotFound, Message: "method not found: " + method}
	}
}

// --- ホスト管理 ---

func (h *Handler) hostList() (any, *RPCError) {
	hosts, err := h.sshMgr.LoadHosts()
	if err != nil {
		return nil, toRPCError(err, InternalError)
	}

	result := HostListResult{
		Hosts: make([]HostInfo, len(hosts)),
	}
	for i, host := range hosts {
		result.Hosts[i] = toHostInfo(host)
	}
	return result, nil
}

func (h *Handler) hostReload() (any, *RPCError) {
	// TODO: ReloadHosts 前後の差分を計算して Added/Removed を返す
	hosts, err := h.sshMgr.ReloadHosts()
	if err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return HostReloadResult{
		Total:   len(hosts),
		Added:   []string{},
		Removed: []string{},
	}, nil
}

// --- SSH 接続管理 ---

// credentialTimeout はクレデンシャル応答のタイムアウト。
const credentialTimeout = 30 * time.Second

func (h *Handler) sshConnect(clientID string, params json.RawMessage) (any, *RPCError) {
	var p SSHConnectParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	// クレデンシャルコールバックを構築
	cb := h.buildCredentialCallback(clientID, p.Host)

	if err := h.sshMgr.ConnectWithCallback(p.Host, cb); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return SSHConnectResult{
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
		ch := make(chan CredentialResponseParams, 1)
		h.credMu.Lock()
		h.credPending[reqID] = ch
		h.credMu.Unlock()

		defer func() {
			h.credMu.Lock()
			delete(h.credPending, reqID)
			h.credMu.Unlock()
		}()

		// credential.request 通知をクライアントに送信
		notif := CredentialRequestNotification{
			RequestID: reqID,
			Type:      string(req.Type),
			Host:      req.Host,
			Prompt:    req.Prompt,
		}
		if len(req.Prompts) > 0 {
			notif.Prompts = make([]PromptData, len(req.Prompts))
			for i, p := range req.Prompts {
				notif.Prompts[i] = PromptData{Prompt: p.Prompt, Echo: p.Echo}
			}
		}

		data, err := json.Marshal(notif)
		if err != nil {
			return core.CredentialResponse{}, fmt.Errorf("marshal credential request: %w", err)
		}

		if err := h.sender.SendNotification(clientID, Notification{
			JSONRPC: JSONRPCVersion,
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
func (h *Handler) credentialResponse(params json.RawMessage) (any, *RPCError) {
	var p CredentialResponseParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	h.credMu.Lock()
	ch, ok := h.credPending[p.RequestID]
	h.credMu.Unlock()

	if !ok {
		return nil, &RPCError{Code: InvalidParams, Message: "no pending credential request for id: " + p.RequestID}
	}

	// 非ブロッキングで送信（チャネルはバッファ1）
	select {
	case ch <- p:
	default:
	}

	return CredentialResponseResult{OK: true}, nil
}

func (h *Handler) sshDisconnect(params json.RawMessage) (any, *RPCError) {
	var p SSHDisconnectParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.sshMgr.Disconnect(p.Host); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return SSHDisconnectResult{
		Host:   p.Host,
		Status: "disconnected",
	}, nil
}

// --- ポートフォワーディング管理 ---

func (h *Handler) forwardList(params json.RawMessage) (any, *RPCError) {
	var p ForwardListParams
	// params が nil や空の場合はデフォルト値を使用する
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			slog.Debug("forwardList: invalid params, using defaults", "error", err)
		}
	}

	var rules []core.ForwardRule
	if p.Host != "" {
		rules = h.fwdMgr.GetRulesByHost(p.Host)
	} else {
		rules = h.fwdMgr.GetRules()
	}

	result := ForwardListResult{
		Forwards: make([]ForwardInfo, len(rules)),
	}
	for i, rule := range rules {
		result.Forwards[i] = toForwardInfo(rule)
	}
	return result, nil
}

func (h *Handler) forwardAdd(params json.RawMessage) (any, *RPCError) {
	var p ForwardAddParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	fwdType, err := core.ParseForwardType(p.Type)
	if err != nil {
		return nil, &RPCError{Code: InvalidParams, Message: err.Error()}
	}

	rule := core.ForwardRule{
		Name:        p.Name,
		Host:        p.Host,
		Type:        fwdType,
		LocalPort:   p.LocalPort,
		RemoteHost:  p.RemoteHost,
		RemotePort:  p.RemotePort,
		AutoConnect: p.AutoConnect,
	}

	name, err := h.fwdMgr.AddRule(rule)
	if err != nil {
		return nil, toRPCError(err, InternalError)
	}

	h.saveForwardRulesToConfig()
	return ForwardAddResult{Name: name}, nil
}

func (h *Handler) forwardDelete(params json.RawMessage) (any, *RPCError) {
	var p ForwardDeleteParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.fwdMgr.DeleteRule(p.Name); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	h.saveForwardRulesToConfig()
	return ForwardDeleteResult{OK: true}, nil
}

func (h *Handler) forwardStart(params json.RawMessage) (any, *RPCError) {
	var p ForwardStartParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.fwdMgr.StartForward(p.Name); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return ForwardStartResult{
		Name:   p.Name,
		Status: "active",
	}, nil
}

func (h *Handler) forwardStop(params json.RawMessage) (any, *RPCError) {
	var p ForwardStopParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.fwdMgr.StopForward(p.Name); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return ForwardStopResult{
		Name:   p.Name,
		Status: "stopped",
	}, nil
}

func (h *Handler) forwardStopAll() (any, *RPCError) {
	sessions := h.fwdMgr.GetAllSessions()
	active := 0
	for _, s := range sessions {
		if s.Status == core.Active {
			active++
		}
	}

	if err := h.fwdMgr.StopAllForwards(); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return ForwardStopAllResult{Stopped: active}, nil
}

// --- セッション情報 ---

func (h *Handler) sessionList() (any, *RPCError) {
	sessions := h.fwdMgr.GetAllSessions()

	result := SessionListResult{
		Sessions: make([]SessionInfo, len(sessions)),
	}
	for i, s := range sessions {
		result.Sessions[i] = toSessionInfo(s)
	}
	return result, nil
}

func (h *Handler) sessionGet(params json.RawMessage) (any, *RPCError) {
	var p SessionGetParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	session, err := h.fwdMgr.GetSession(p.Name)
	if err != nil {
		return nil, toRPCError(err, InternalError)
	}

	info := toSessionInfo(*session)
	return info, nil
}

// --- 設定管理 ---

func (h *Handler) configGet() (any, *RPCError) {
	cfg := h.cfgMgr.GetConfig()

	return ConfigGetResult{
		SSHConfigPath: cfg.SSHConfigPath,
		Reconnect: ReconnectInfo{
			Enabled:      cfg.Reconnect.Enabled,
			MaxRetries:   cfg.Reconnect.MaxRetries,
			InitialDelay: cfg.Reconnect.InitialDelay.Duration.String(),
			MaxDelay:     cfg.Reconnect.MaxDelay.Duration.String(),
		},
		Session: SessionCfgInfo{
			AutoRestore: cfg.Session.AutoRestore,
		},
		Log: LogInfo{
			Level: cfg.Log.Level,
			File:  cfg.Log.File,
		},
	}, nil
}

func (h *Handler) configUpdate(params json.RawMessage) (any, *RPCError) {
	var p ConfigUpdateParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.cfgMgr.UpdateConfig(func(cfg *core.Config) {
		if p.SSHConfigPath != nil {
			cfg.SSHConfigPath = *p.SSHConfigPath
		}
		if p.Reconnect != nil {
			if p.Reconnect.Enabled != nil {
				cfg.Reconnect.Enabled = *p.Reconnect.Enabled
			}
			if p.Reconnect.MaxRetries != nil {
				cfg.Reconnect.MaxRetries = *p.Reconnect.MaxRetries
			}
			if p.Reconnect.InitialDelay != nil {
				if d, err := time.ParseDuration(*p.Reconnect.InitialDelay); err == nil {
					cfg.Reconnect.InitialDelay = core.Duration{Duration: d}
				}
			}
			if p.Reconnect.MaxDelay != nil {
				if d, err := time.ParseDuration(*p.Reconnect.MaxDelay); err == nil {
					cfg.Reconnect.MaxDelay = core.Duration{Duration: d}
				}
			}
		}
		if p.Session != nil && p.Session.AutoRestore != nil {
			cfg.Session.AutoRestore = *p.Session.AutoRestore
		}
		if p.Log != nil {
			if p.Log.Level != nil {
				cfg.Log.Level = *p.Log.Level
			}
			if p.Log.File != nil {
				cfg.Log.File = *p.Log.File
			}
		}
	}); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return ConfigUpdateResult{OK: true}, nil
}

// --- デーモン管理 ---

func (h *Handler) daemonStatus() (any, *RPCError) {
	if h.daemon == nil {
		return nil, &RPCError{Code: InternalError, Message: "daemon not available"}
	}
	return h.daemon.Status(), nil
}

func (h *Handler) daemonShutdown(params json.RawMessage) (any, *RPCError) {
	if h.daemon == nil {
		return nil, &RPCError{Code: InternalError, Message: "daemon not available"}
	}

	var p DaemonShutdownParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			slog.Debug("daemonShutdown: invalid params, using defaults", "error", err)
		}
	}

	if err := h.daemon.Shutdown(p.Purge); err != nil {
		return nil, toRPCError(err, InternalError)
	}
	return DaemonShutdownResult{OK: true}, nil
}

// --- イベントサブスクリプション ---

// validEventTypes は有効なイベント種別。
var validEventTypes = map[string]bool{
	"ssh":     true,
	"forward": true,
	"metrics": true,
}

func (h *Handler) eventsSubscribe(clientID string, params json.RawMessage) (any, *RPCError) {
	var p EventsSubscribeParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	for _, t := range p.Types {
		if !validEventTypes[t] {
			return nil, &RPCError{Code: InvalidParams, Message: "invalid event type: " + t}
		}
	}

	subID := h.broker.Subscribe(clientID, p.Types)
	return EventsSubscribeResult{SubscriptionID: subID}, nil
}

func (h *Handler) eventsUnsubscribe(params json.RawMessage) (any, *RPCError) {
	var p EventsUnsubscribeParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if !h.broker.Unsubscribe(p.SubscriptionID) {
		return nil, &RPCError{Code: InvalidParams, Message: "subscription not found"}
	}

	return EventsUnsubscribeResult{OK: true}, nil
}

// --- ヘルパー関数 ---

// saveForwardRulesToConfig はフォワードルールを設定ファイルに保存する。
func (h *Handler) saveForwardRulesToConfig() {
	rules := h.fwdMgr.GetRules()
	if err := h.cfgMgr.UpdateConfig(func(c *core.Config) {
		c.Forwards = rules
	}); err != nil {
		slog.Warn("failed to save forward rules to config", "error", err)
	}
}

// parseParams は JSON-RPC パラメータをアンマーシャルする。
func parseParams(params json.RawMessage, target any) *RPCError {
	if len(params) == 0 {
		return &RPCError{Code: InvalidParams, Message: "params required"}
	}
	if err := json.Unmarshal(params, target); err != nil {
		return &RPCError{Code: InvalidParams, Message: "invalid params: " + err.Error()}
	}
	return nil
}
