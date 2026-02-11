package ipc

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// DaemonInfo はデーモンの状態情報とシャットダウンを提供するインターフェース。
type DaemonInfo interface {
	Status() DaemonStatusResult
	Shutdown() error
}

// Handler は JSON-RPC メソッドをコアマネージャーにルーティングする。
type Handler struct {
	sshMgr core.SSHManager
	fwdMgr core.ForwardManager
	cfgMgr core.ConfigManager
	broker *EventBroker
	daemon DaemonInfo
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
		sshMgr: sshMgr,
		fwdMgr: fwdMgr,
		cfgMgr: cfgMgr,
		broker: broker,
		daemon: daemon,
	}
}

// Handle は JSON-RPC メソッドをディスパッチする。HandlerFunc として使用する。
func (h *Handler) Handle(clientID string, method string, params json.RawMessage) (any, *RPCError) {
	switch method {
	case "host.list":
		return h.hostList()
	case "host.reload":
		return h.hostReload()
	case "ssh.connect":
		return h.sshConnect(params)
	case "ssh.disconnect":
		return h.sshDisconnect(params)
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
		return h.daemonShutdown()
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

func (h *Handler) sshConnect(params json.RawMessage) (any, *RPCError) {
	var p SSHConnectParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.sshMgr.Connect(p.Host); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	return SSHConnectResult{
		Host:   p.Host,
		Status: "connected",
	}, nil
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
	// params が nil や空の場合もあるため、エラーを無視する
	if len(params) > 0 {
		json.Unmarshal(params, &p)
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

	if err := h.fwdMgr.AddRule(rule); err != nil {
		return nil, toRPCError(err, InternalError)
	}

	// AddRule が名前を自動生成する場合があるため、rule.Name が空の場合は
	// GetRules から取得する必要があるが、ここでは入力名を返す
	name := p.Name
	if name == "" {
		// 自動生成された名前を取得するため、最新のルール一覧から取得
		rules := h.fwdMgr.GetRules()
		if len(rules) > 0 {
			name = rules[len(rules)-1].Name
		}
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

func (h *Handler) daemonShutdown() (any, *RPCError) {
	if h.daemon == nil {
		return nil, &RPCError{Code: InternalError, Message: "daemon not available"}
	}
	if err := h.daemon.Shutdown(); err != nil {
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
	_ = h.cfgMgr.UpdateConfig(func(c *core.Config) {
		c.Forwards = rules
	})
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

// toRPCError はコアエラーを RPCError に変換する。
// エラーメッセージに基づいてアプリケーション固有のエラーコードを割り当てる。
func toRPCError(err error, defaultCode int) *RPCError {
	msg := err.Error()

	switch {
	case strings.Contains(msg, "not found"):
		if strings.Contains(msg, "host") {
			return &RPCError{Code: HostNotFound, Message: msg}
		}
		if strings.Contains(msg, "rule") {
			return &RPCError{Code: RuleNotFound, Message: msg}
		}
	case strings.Contains(msg, "already exists"):
		return &RPCError{Code: RuleAlreadyExists, Message: msg}
	case strings.Contains(msg, "already active"):
		return &RPCError{Code: AlreadyConnected, Message: msg}
	case strings.Contains(msg, "not connected"):
		return &RPCError{Code: NotConnected, Message: msg}
	case strings.Contains(msg, "already connected"):
		return &RPCError{Code: AlreadyConnected, Message: msg}
	}

	return &RPCError{Code: defaultCode, Message: msg}
}

// toHostInfo は core.SSHHost を HostInfo に変換する。
func toHostInfo(host core.SSHHost) HostInfo {
	return HostInfo{
		Name:               host.Name,
		HostName:           host.HostName,
		Port:               host.Port,
		User:               host.User,
		State:              strings.ToLower(host.State.String()),
		ActiveForwardCount: host.ActiveForwardCount,
	}
}

// toForwardInfo は core.ForwardRule を ForwardInfo に変換する。
func toForwardInfo(rule core.ForwardRule) ForwardInfo {
	return ForwardInfo{
		Name:        rule.Name,
		Host:        rule.Host,
		Type:        strings.ToLower(rule.Type.String()),
		LocalPort:   rule.LocalPort,
		RemoteHost:  rule.RemoteHost,
		RemotePort:  rule.RemotePort,
		AutoConnect: rule.AutoConnect,
	}
}

// toSessionInfo は core.ForwardSession を SessionInfo に変換する。
func toSessionInfo(s core.ForwardSession) SessionInfo {
	info := SessionInfo{
		ID:             s.ID,
		Name:           s.Rule.Name,
		Host:           s.Rule.Host,
		Type:           strings.ToLower(s.Rule.Type.String()),
		LocalPort:      s.Rule.LocalPort,
		RemoteHost:     s.Rule.RemoteHost,
		RemotePort:     s.Rule.RemotePort,
		Status:         strings.ToLower(s.Status.String()),
		BytesSent:      s.BytesSent,
		BytesReceived:  s.BytesReceived,
		ReconnectCount: s.ReconnectCount,
		LastError:      s.LastError,
	}
	if !s.ConnectedAt.IsZero() {
		info.ConnectedAt = s.ConnectedAt.Format(time.RFC3339)
	}
	return info
}
