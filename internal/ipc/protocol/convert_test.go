package protocol

import (
	"fmt"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestToRPCError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		defaultCode int
		wantCode    int
		wantMsg     string
	}{
		{
			name:        "host not found",
			err:         fmt.Errorf("host not found"),
			defaultCode: InternalError,
			wantCode:    HostNotFound,
			wantMsg:     "host not found",
		},
		{
			name:        "rule not found",
			err:         fmt.Errorf("rule not found"),
			defaultCode: InternalError,
			wantCode:    RuleNotFound,
			wantMsg:     "rule not found",
		},
		{
			name:        "already exists",
			err:         fmt.Errorf("rule already exists"),
			defaultCode: InternalError,
			wantCode:    RuleAlreadyExists,
			wantMsg:     "rule already exists",
		},
		{
			name:        "already active",
			err:         fmt.Errorf("connection already active"),
			defaultCode: InternalError,
			wantCode:    AlreadyConnected,
			wantMsg:     "connection already active",
		},
		{
			name:        "not connected",
			err:         fmt.Errorf("host is not connected"),
			defaultCode: InternalError,
			wantCode:    NotConnected,
			wantMsg:     "host is not connected",
		},
		{
			name:        "already connected",
			err:         fmt.Errorf("host already connected"),
			defaultCode: InternalError,
			wantCode:    AlreadyConnected,
			wantMsg:     "host already connected",
		},
		{
			name:        "credential timeout",
			err:         fmt.Errorf("credential timeout"),
			defaultCode: InternalError,
			wantCode:    CredentialTimeout,
			wantMsg:     "credential timeout",
		},
		{
			name:        "credential cancelled",
			err:         fmt.Errorf("credential cancelled"),
			defaultCode: InternalError,
			wantCode:    CredentialCancelled,
			wantMsg:     "credential cancelled",
		},
		{
			name:        "address already in use",
			err:         fmt.Errorf("listen tcp :8080: bind: address already in use"),
			defaultCode: InternalError,
			wantCode:    PortConflict,
			wantMsg:     "listen tcp :8080: bind: address already in use",
		},
		{
			name:        "authentication required",
			err:         fmt.Errorf("authentication required"),
			defaultCode: InternalError,
			wantCode:    AuthenticationFailed,
			wantMsg:     "authentication required",
		},
		{
			name:        "unable to authenticate",
			err:         fmt.Errorf("ssh: unable to authenticate"),
			defaultCode: InternalError,
			wantCode:    AuthenticationFailed,
			wantMsg:     "ssh: unable to authenticate",
		},
		{
			name:        "no authentication methods available",
			err:         fmt.Errorf("ssh: no authentication methods available"),
			defaultCode: InternalError,
			wantCode:    AuthenticationFailed,
			wantMsg:     "ssh: no authentication methods available",
		},
		{
			name:        "no supported methods remain",
			err:         fmt.Errorf("ssh: no supported methods remain"),
			defaultCode: InternalError,
			wantCode:    AuthenticationFailed,
			wantMsg:     "ssh: no supported methods remain",
		},
		{
			name:        "generic error uses defaultCode",
			err:         fmt.Errorf("something unexpected happened"),
			defaultCode: InvalidParams,
			wantCode:    InvalidParams,
			wantMsg:     "something unexpected happened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToRPCError(tt.err, tt.defaultCode)
			if got.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", got.Code, tt.wantCode)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
		})
	}
}

func TestToHostInfo(t *testing.T) {
	tests := []struct {
		name string
		host core.SSHHost
		want HostInfo
	}{
		{"connected host", core.SSHHost{
			Name: "prod", HostName: "192.168.1.1", Port: 22, User: "admin",
			State: core.Connected, ActiveForwardCount: 3,
		}, HostInfo{
			Name: "prod", HostName: "192.168.1.1", Port: 22, User: "admin",
			State: "connected", ActiveForwardCount: 3,
		}},
		{"disconnected host", core.SSHHost{
			Name: "staging", HostName: "10.0.0.1", Port: 2222, User: "deploy",
			State: core.Disconnected,
		}, HostInfo{
			Name: "staging", HostName: "10.0.0.1", Port: 2222, User: "deploy",
			State: "disconnected",
		}},
		{"pending_auth host uses snake_case wire format", core.SSHHost{
			Name: "auth-host", HostName: "10.0.0.2", Port: 22, User: "user",
			State: core.PendingAuth,
		}, HostInfo{
			Name: "auth-host", HostName: "10.0.0.2", Port: 22, User: "user",
			State: "pending_auth",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToHostInfo(tt.host)
			if got != tt.want {
				t.Errorf("ToHostInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToForwardInfo(t *testing.T) {
	tests := []struct {
		name string
		rule core.ForwardRule
		want ForwardInfo
	}{
		{"local forward rule", core.ForwardRule{
			Name: "web", Host: "prod", Type: core.Local,
			LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80, AutoConnect: true,
		}, ForwardInfo{
			Name: "web", Host: "prod", Type: "local",
			LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80, AutoConnect: true,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToForwardInfo(tt.rule)
			if got != tt.want {
				t.Errorf("ToForwardInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToSessionInfo(t *testing.T) {
	connectedAt := time.Date(2026, 2, 11, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		sess core.ForwardSession
		want SessionInfo
	}{
		{"non-zero ConnectedAt formatted as RFC3339", core.ForwardSession{
			ID:     "prod-local-8080",
			Rule:   core.ForwardRule{Name: "web", Host: "prod", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
			Status: core.Active, ConnectedAt: connectedAt,
			BytesSent: 1024, BytesReceived: 2048, ReconnectCount: 1, LastError: "connection reset",
		}, SessionInfo{
			ID: "prod-local-8080", Name: "web", Host: "prod", Type: "local",
			LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
			Status: "active", ConnectedAt: connectedAt.Format(time.RFC3339),
			BytesSent: 1024, BytesReceived: 2048, ReconnectCount: 1, LastError: "connection reset",
		}},
		{"zero ConnectedAt results in empty string", core.ForwardSession{
			ID:     "staging-local-3000",
			Rule:   core.ForwardRule{Name: "api", Host: "staging", Type: core.Local, LocalPort: 3000, RemoteHost: "localhost", RemotePort: 3000},
			Status: core.Stopped, ConnectedAt: time.Time{},
		}, SessionInfo{
			ID: "staging-local-3000", Name: "api", Host: "staging", Type: "local",
			LocalPort: 3000, RemoteHost: "localhost", RemotePort: 3000, Status: "stopped",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToSessionInfo(tt.sess)
			if got != tt.want {
				t.Errorf("ToSessionInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
