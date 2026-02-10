package atoms_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
)

func TestRenderConnectionBadge(t *testing.T) {
	tests := []struct {
		name       string
		state      core.ConnectionState
		wantSymbol string
		wantLabel  string
	}{
		{"Connected", core.Connected, "●", "Connected"},
		{"Disconnected", core.Disconnected, "○", "Disconnected"},
		{"Error", core.ConnectionError, "✗", "Error"},
		{"Reconnecting", core.Reconnecting, "↻", "Reconnecting"},
		{"Connecting", core.Connecting, "◐", "Connecting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atoms.RenderConnectionBadge(tt.state)
			if !strings.Contains(got, tt.wantSymbol) {
				t.Errorf("RenderConnectionBadge(%v) = %q, want symbol %q", tt.state, got, tt.wantSymbol)
			}
			if !strings.Contains(got, tt.wantLabel) {
				t.Errorf("RenderConnectionBadge(%v) = %q, want label %q", tt.state, got, tt.wantLabel)
			}
		})
	}
}

func TestRenderSessionBadge(t *testing.T) {
	tests := []struct {
		name       string
		status     core.SessionStatus
		wantSymbol string
		wantLabel  string
	}{
		{"Active", core.Active, "●", "Active"},
		{"Stopped", core.Stopped, "○", "Stopped"},
		{"Error", core.SessionError, "✗", "Error"},
		{"Reconnecting", core.SessionReconnecting, "↻", "Reconnecting"},
		{"Starting", core.Starting, "◐", "Starting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atoms.RenderSessionBadge(tt.status)
			if !strings.Contains(got, tt.wantSymbol) {
				t.Errorf("RenderSessionBadge(%v) = %q, want symbol %q", tt.status, got, tt.wantSymbol)
			}
			if !strings.Contains(got, tt.wantLabel) {
				t.Errorf("RenderSessionBadge(%v) = %q, want label %q", tt.status, got, tt.wantLabel)
			}
		})
	}
}

func TestRenderPortLabel(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{8080, ":8080"},
		{22, ":22"},
		{443, ":443"},
		{0, ":0"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := atoms.RenderPortLabel(tt.port)
			if !strings.Contains(got, tt.want) {
				t.Errorf("RenderPortLabel(%d) = %q, want to contain %q", tt.port, got, tt.want)
			}
		})
	}
}

func TestRenderDataSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0B"},
		{"small bytes", 52, "52B"},
		{"kilobytes", 340 * 1024, "340.0KB"},
		{"megabytes", 1258291, "1.2MB"},
		{"gigabytes", 2684354560, "2.5GB"},
		{"exact 1KB", 1024, "1.0KB"},
		{"exact 1MB", 1 << 20, "1.0MB"},
		{"exact 1GB", 1 << 30, "1.0GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atoms.RenderDataSize(tt.bytes)
			if !strings.Contains(got, tt.want) {
				t.Errorf("RenderDataSize(%d) = %q, want to contain %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestRenderDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"seconds only", 30 * time.Second, "30s"},
		{"minutes and seconds", 5*time.Minute + 30*time.Second, "5m 30s"},
		{"hours and minutes", 2*time.Hour + 15*time.Minute, "2h 15m"},
		{"days and hours", 25 * time.Hour, "1d 1h"},
		{"zero", 0, "0s"},
		{"just under a minute", 59 * time.Second, "59s"},
		{"exactly one hour", 1 * time.Hour, "1h 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atoms.RenderDuration(tt.duration)
			if !strings.Contains(got, tt.want) {
				t.Errorf("RenderDuration(%v) = %q, want to contain %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestRenderDivider(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  string
	}{
		{"normal width", 10, "──────────"},
		{"width 1", 1, "─"},
		{"width 0", 0, ""},
		{"negative width", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atoms.RenderDivider(tt.width)
			if tt.width <= 0 {
				if got != "" {
					t.Errorf("RenderDivider(%d) = %q, want empty string", tt.width, got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("RenderDivider(%d) = %q, want to contain %q", tt.width, got, tt.want)
			}
		})
	}
}
