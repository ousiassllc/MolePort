package app

import (
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// hostInfoToSSHHost は IPC の HostInfo を core.SSHHost に変換する。
func hostInfoToSSHHost(info protocol.HostInfo) core.SSHHost {
	return core.SSHHost{
		Name:               info.Name,
		HostName:           info.HostName,
		Port:               info.Port,
		User:               info.User,
		State:              protocol.ParseConnectionState(info.State),
		ActiveForwardCount: info.ActiveForwardCount,
	}
}

// sessionInfoToForwardSession は IPC の SessionInfo を core.ForwardSession に変換する。
func sessionInfoToForwardSession(info protocol.SessionInfo) core.ForwardSession {
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

func parseSessionStatus(s string) core.SessionStatus {
	switch s {
	case protocol.SessionActive:
		return core.Active
	case protocol.SessionStarting:
		return core.Starting
	case protocol.SessionReconnecting:
		return core.SessionReconnecting
	case protocol.SessionError:
		return core.SessionError
	default:
		return core.Stopped
	}
}
