package core

import "testing"

func TestSSHEventType_String(t *testing.T) {
	tests := []struct {
		et   SSHEventType
		want string
	}{
		{SSHEventConnected, "Connected"},
		{SSHEventDisconnected, "Disconnected"},
		{SSHEventReconnecting, "Reconnecting"},
		{SSHEventPendingAuth, "PendingAuth"},
		{SSHEventError, "Error"},
		{SSHEventType(99), "SSHEventType(99)"},
	}
	for _, tt := range tests {
		if got := tt.et.String(); got != tt.want {
			t.Errorf("SSHEventType(%d).String() = %q, want %q", int(tt.et), got, tt.want)
		}
	}
}

func TestForwardEventType_String(t *testing.T) {
	tests := []struct {
		et   ForwardEventType
		want string
	}{
		{ForwardEventStarted, "Started"},
		{ForwardEventStopped, "Stopped"},
		{ForwardEventError, "Error"},
		{ForwardEventMetricsUpdated, "MetricsUpdated"},
		{ForwardEventType(99), "ForwardEventType(99)"},
	}
	for _, tt := range tests {
		if got := tt.et.String(); got != tt.want {
			t.Errorf("ForwardEventType(%d).String() = %q, want %q", int(tt.et), got, tt.want)
		}
	}
}
