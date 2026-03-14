package protocol

import (
	"errors"
	"strings"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// ToRPCError はコアエラーを RPCError に変換する。
// 構造化エラー型に基づいてアプリケーション固有のエラーコードを割り当てる。
// 外部起因エラーについては文字列マッチによるフォールバックを使用する。
func ToRPCError(err error, defaultCode int) *RPCError {
	msg := err.Error()

	// センチネルエラー
	switch {
	case errors.Is(err, core.ErrCredentialTimeout):
		return &RPCError{Code: CredentialTimeout, Message: msg}
	case errors.Is(err, core.ErrCredentialCancelled):
		return &RPCError{Code: CredentialCancelled, Message: msg}
	}

	// 構造化エラー型
	var notFound *core.NotFoundError
	if errors.As(err, &notFound) {
		switch notFound.Resource {
		case "host":
			return &RPCError{Code: HostNotFound, Message: msg}
		case "rule":
			return &RPCError{Code: RuleNotFound, Message: msg}
		}
	}

	var alreadyExists *core.AlreadyExistsError
	if errors.As(err, &alreadyExists) {
		return &RPCError{Code: RuleAlreadyExists, Message: msg}
	}

	var alreadyActive *core.AlreadyActiveError
	if errors.As(err, &alreadyActive) {
		return &RPCError{Code: AlreadyConnected, Message: msg}
	}

	var notConnected *core.NotConnectedError
	if errors.As(err, &notConnected) {
		return &RPCError{Code: NotConnected, Message: msg}
	}

	var authRequired *core.AuthRequiredError
	if errors.As(err, &authRequired) {
		return &RPCError{Code: AuthenticationFailed, Message: msg}
	}

	// 外部起因エラー: 文字列マッチによるフォールバック
	switch {
	case strings.Contains(msg, "address already in use"):
		return &RPCError{Code: PortConflict, Message: msg}
	case core.IsAuthFailure(err):
		return &RPCError{Code: AuthenticationFailed, Message: msg}
	}

	return &RPCError{Code: defaultCode, Message: msg}
}

// ToHostInfo は core.SSHHost を HostInfo に変換する。
func ToHostInfo(host core.SSHHost) HostInfo {
	return HostInfo{
		Name:               host.Name,
		HostName:           host.HostName,
		Port:               host.Port,
		User:               host.User,
		State:              connectionStateToWire(host.State),
		ActiveForwardCount: host.ActiveForwardCount,
	}
}

// ToForwardInfo は core.ForwardRule を ForwardInfo に変換する。
func ToForwardInfo(rule core.ForwardRule) ForwardInfo {
	return ForwardInfo{
		Name:           rule.Name,
		Host:           rule.Host,
		Type:           forwardTypeToWire(rule.Type),
		LocalPort:      rule.LocalPort,
		RemoteHost:     rule.RemoteHost,
		RemotePort:     rule.RemotePort,
		RemoteBindAddr: rule.RemoteBindAddr,
		AutoConnect:    rule.AutoConnect,
	}
}

// ToSessionInfo は core.ForwardSession を SessionInfo に変換する。
func ToSessionInfo(s core.ForwardSession) SessionInfo {
	info := SessionInfo{
		ID:             s.ID,
		Name:           s.Rule.Name,
		Host:           s.Rule.Host,
		Type:           forwardTypeToWire(s.Rule.Type),
		LocalPort:      s.Rule.LocalPort,
		RemoteHost:     s.Rule.RemoteHost,
		RemotePort:     s.Rule.RemotePort,
		RemoteBindAddr: s.Rule.RemoteBindAddr,
		Status:         sessionStatusToWire(s.Status),
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

// connectionStateToWire は core.ConnectionState を IPC ワイヤー文字列に変換する。
func connectionStateToWire(s core.ConnectionState) string {
	switch s {
	case core.Connected:
		return StateConnected
	case core.Connecting:
		return StateConnecting
	case core.Reconnecting:
		return StateReconnecting
	case core.PendingAuth:
		return StatePendingAuth
	case core.ConnectionError:
		return StateError
	default:
		return StateDisconnected
	}
}

// sessionStatusToWire は core.SessionStatus を IPC ワイヤー文字列に変換する。
func sessionStatusToWire(s core.SessionStatus) string {
	switch s {
	case core.Active:
		return SessionActive
	case core.Starting:
		return SessionStarting
	case core.SessionReconnecting:
		return SessionReconnecting
	case core.SessionError:
		return SessionError
	default:
		return SessionStopped
	}
}

// ParseConnectionState は IPC ワイヤー文字列を core.ConnectionState に変換する。
func ParseConnectionState(s string) core.ConnectionState {
	switch s {
	case StateConnected:
		return core.Connected
	case StateConnecting:
		return core.Connecting
	case StateReconnecting:
		return core.Reconnecting
	case StatePendingAuth:
		return core.PendingAuth
	case StateError:
		return core.ConnectionError
	default:
		return core.Disconnected
	}
}

// forwardTypeToWire は core.ForwardType を IPC ワイヤー文字列に変換する。
func forwardTypeToWire(t core.ForwardType) string {
	switch t {
	case core.Local:
		return ForwardTypeLocal
	case core.Remote:
		return ForwardTypeRemote
	case core.Dynamic:
		return ForwardTypeDynamic
	default:
		return ForwardTypeLocal
	}
}
