package core

import "fmt"

// SSHEventType は SSH イベントの種別を表す。
type SSHEventType int

const (
	SSHEventConnected SSHEventType = iota
	SSHEventDisconnected
	SSHEventReconnecting
	SSHEventPendingAuth
	SSHEventError
)

func (t SSHEventType) String() string {
	switch t {
	case SSHEventConnected:
		return "Connected"
	case SSHEventDisconnected:
		return "Disconnected"
	case SSHEventReconnecting:
		return "Reconnecting"
	case SSHEventPendingAuth:
		return "PendingAuth"
	case SSHEventError:
		return "Error"
	default:
		return fmt.Sprintf("SSHEventType(%d)", int(t))
	}
}

// SSHEvent は SSH 接続に関するイベント。
type SSHEvent struct {
	Type     SSHEventType
	HostName string
	Error    error
}

// ForwardEventType はポートフォワーディングイベントの種別を表す。
type ForwardEventType int

const (
	ForwardEventStarted ForwardEventType = iota
	ForwardEventStopped
	ForwardEventError
	ForwardEventMetricsUpdated
	ForwardEventReconnecting // SSH 接続断によりフォワードが再接続待ち
	ForwardEventRestored     // SSH 再接続後にフォワードが自動復元
)

func (t ForwardEventType) String() string {
	switch t {
	case ForwardEventStarted:
		return "Started"
	case ForwardEventStopped:
		return "Stopped"
	case ForwardEventError:
		return "Error"
	case ForwardEventMetricsUpdated:
		return "MetricsUpdated"
	case ForwardEventReconnecting:
		return "Reconnecting"
	case ForwardEventRestored:
		return "Restored"
	default:
		return fmt.Sprintf("ForwardEventType(%d)", int(t))
	}
}

// ForwardEvent はポートフォワーディングに関するイベント。
type ForwardEvent struct {
	Type     ForwardEventType
	RuleName string
	Session  *ForwardSession
	Error    error
}
