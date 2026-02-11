package app

import (
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
)

// hostInfoToSSHHost は IPC の HostInfo を core.SSHHost に変換する。
func hostInfoToSSHHost(info ipc.HostInfo) core.SSHHost {
	return core.SSHHost{
		Name:               info.Name,
		HostName:           info.HostName,
		Port:               info.Port,
		User:               info.User,
		State:              parseConnectionState(info.State),
		ActiveForwardCount: info.ActiveForwardCount,
	}
}

// sessionInfoToForwardSession は IPC の SessionInfo を core.ForwardSession に変換する。
func sessionInfoToForwardSession(info ipc.SessionInfo) core.ForwardSession {
	fwdType, _ := core.ParseForwardType(info.Type)
	status := parseSessionStatus(info.Status)
	var connectedAt time.Time
	if info.ConnectedAt != "" {
		connectedAt, _ = time.Parse(time.RFC3339, info.ConnectedAt)
	}
	return core.ForwardSession{
		ID: info.ID,
		Rule: core.ForwardRule{
			Name:       info.Name,
			Host:       info.Host,
			Type:       fwdType,
			LocalPort:  info.LocalPort,
			RemoteHost: info.RemoteHost,
			RemotePort: info.RemotePort,
		},
		Status:         status,
		ConnectedAt:    connectedAt,
		BytesSent:      info.BytesSent,
		BytesReceived:  info.BytesReceived,
		ReconnectCount: info.ReconnectCount,
		LastError:      info.LastError,
	}
}

func parseConnectionState(s string) core.ConnectionState {
	switch s {
	case "connected":
		return core.Connected
	case "connecting":
		return core.Connecting
	case "reconnecting":
		return core.Reconnecting
	case "pending_auth":
		return core.PendingAuth
	case "error":
		return core.ConnectionError
	default:
		return core.Disconnected
	}
}

func parseSessionStatus(s string) core.SessionStatus {
	switch s {
	case "active":
		return core.Active
	case "starting":
		return core.Starting
	case "reconnecting":
		return core.SessionReconnecting
	case "error":
		return core.SessionError
	default:
		return core.Stopped
	}
}
