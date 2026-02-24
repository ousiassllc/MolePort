package ipc

import (
	"strings"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// toRPCError はコアエラーを RPCError に変換する。
// エラーメッセージに基づいてアプリケーション固有のエラーコードを割り当てる。
func toRPCError(err error, defaultCode int) *protocol.RPCError {
	msg := err.Error()

	switch {
	case strings.Contains(msg, "not found"):
		if strings.Contains(msg, "host") {
			return &protocol.RPCError{Code: protocol.HostNotFound, Message: msg}
		}
		if strings.Contains(msg, "rule") {
			return &protocol.RPCError{Code: protocol.RuleNotFound, Message: msg}
		}
	case strings.Contains(msg, "already exists"):
		return &protocol.RPCError{Code: protocol.RuleAlreadyExists, Message: msg}
	case strings.Contains(msg, "already active"):
		return &protocol.RPCError{Code: protocol.AlreadyConnected, Message: msg}
	case strings.Contains(msg, "not connected"):
		return &protocol.RPCError{Code: protocol.NotConnected, Message: msg}
	case strings.Contains(msg, "already connected"):
		return &protocol.RPCError{Code: protocol.AlreadyConnected, Message: msg}
	case strings.Contains(msg, "credential timeout"):
		return &protocol.RPCError{Code: protocol.CredentialTimeout, Message: msg}
	case strings.Contains(msg, "credential cancelled"):
		return &protocol.RPCError{Code: protocol.CredentialCancelled, Message: msg}
	}

	return &protocol.RPCError{Code: defaultCode, Message: msg}
}

// toHostInfo は core.SSHHost を HostInfo に変換する。
func toHostInfo(host core.SSHHost) protocol.HostInfo {
	return protocol.HostInfo{
		Name:               host.Name,
		HostName:           host.HostName,
		Port:               host.Port,
		User:               host.User,
		State:              strings.ToLower(host.State.String()),
		ActiveForwardCount: host.ActiveForwardCount,
	}
}

// toForwardInfo は core.ForwardRule を ForwardInfo に変換する。
func toForwardInfo(rule core.ForwardRule) protocol.ForwardInfo {
	return protocol.ForwardInfo{
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
func toSessionInfo(s core.ForwardSession) protocol.SessionInfo {
	info := protocol.SessionInfo{
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
