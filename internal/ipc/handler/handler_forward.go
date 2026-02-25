package handler

import (
	"encoding/json"
	"log/slog"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func (h *Handler) forwardList(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.ForwardListParams
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

	result := protocol.ForwardListResult{
		Forwards: make([]protocol.ForwardInfo, len(rules)),
	}
	for i, rule := range rules {
		result.Forwards[i] = protocol.ToForwardInfo(rule)
	}
	return result, nil
}

func (h *Handler) forwardAdd(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.ForwardAddParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	fwdType, err := core.ParseForwardType(p.Type)
	if err != nil {
		return nil, &protocol.RPCError{Code: protocol.InvalidParams, Message: err.Error()}
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
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	h.saveForwardRulesToConfig()
	return protocol.ForwardAddResult{Name: name}, nil
}

func (h *Handler) forwardDelete(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.ForwardDeleteParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.fwdMgr.DeleteRule(p.Name); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	h.saveForwardRulesToConfig()
	return protocol.ForwardDeleteResult{OK: true}, nil
}

func (h *Handler) forwardStart(clientID string, params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.ForwardStartParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	// ホスト未接続の場合、クレデンシャルコールバック付きで事前接続する。
	// これにより forward.start でパスワード認証もサポートされる。
	// 注意: StartForward 内にも Connect のフォールバックがあるが、
	// そちらはコールバックなしのため、パスワード認証が必要な場合は
	// ここでの事前接続が必須。
	session, err := h.fwdMgr.GetSession(p.Name)
	if err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}
	if !h.sshMgr.IsConnected(session.Rule.Host) {
		cb := h.buildCredentialCallback(clientID, session.Rule.Host)
		if err := h.sshMgr.ConnectWithCallback(session.Rule.Host, cb); err != nil {
			return nil, protocol.ToRPCError(err, protocol.InternalError)
		}
	}

	if err := h.fwdMgr.StartForward(p.Name); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.ForwardStartResult{
		Name:   p.Name,
		Status: "active",
	}, nil
}

func (h *Handler) forwardStop(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.ForwardStopParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.fwdMgr.StopForward(p.Name); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.ForwardStopResult{
		Name:   p.Name,
		Status: "stopped",
	}, nil
}

func (h *Handler) forwardStopAll() (any, *protocol.RPCError) {
	sessions := h.fwdMgr.GetAllSessions()
	active := 0
	for _, s := range sessions {
		if s.Status == core.Active {
			active++
		}
	}

	if err := h.fwdMgr.StopAllForwards(); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.ForwardStopAllResult{Stopped: active}, nil
}

// saveForwardRulesToConfig はフォワードルールを設定ファイルに保存する。
func (h *Handler) saveForwardRulesToConfig() {
	rules := h.fwdMgr.GetRules()
	if err := h.cfgMgr.UpdateConfig(func(c *core.Config) {
		c.Forwards = rules
	}); err != nil {
		slog.Warn("failed to save forward rules to config", "error", err)
	}
}
