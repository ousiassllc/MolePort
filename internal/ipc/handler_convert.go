package ipc

import (
	"strings"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

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
	case strings.Contains(msg, "credential timeout"):
		return &RPCError{Code: CredentialTimeout, Message: msg}
	case strings.Contains(msg, "credential cancelled"):
		return &RPCError{Code: CredentialCancelled, Message: msg}
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
