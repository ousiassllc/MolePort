package molecules

import (
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// ---------------------------------------------------------------------------
// forwardTypeLabel
// ---------------------------------------------------------------------------

func TestForwardTypeLabel(t *testing.T) {
	tests := []struct {
		name     string
		ft       core.ForwardType
		expected string
	}{
		{"Local", core.Local, "L"},
		{"Remote", core.Remote, "R"},
		{"Dynamic", core.Dynamic, "D"},
		{"Unknown", core.ForwardType(99), "?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := forwardTypeLabel(tt.ft)
			if got != tt.expected {
				t.Errorf("forwardTypeLabel(%v) = %q, want %q", tt.ft, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ForwardRow.View
// ---------------------------------------------------------------------------

func TestForwardRow_View_ActiveLocal(t *testing.T) {
	row := ForwardRow{
		Session: core.ForwardSession{
			ID: "s1",
			Rule: core.ForwardRule{
				Name:       "web-proxy",
				Type:       core.Local,
				LocalPort:  8080,
				RemoteHost: "localhost",
				RemotePort: 80,
			},
			Status:      core.Active,
			ConnectedAt: time.Now().Add(-10 * time.Minute),
			BytesSent:   1024,
		},
		HostName: "myhost",
		Width:    120,
	}

	out := row.View()
	if out == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(out, "myhost") {
		t.Error("View() should contain host name 'myhost'")
	}
	if !strings.Contains(out, "8080") {
		t.Error("View() should contain local port '8080'")
	}
	if !strings.Contains(out, "web-proxy") {
		t.Error("View() should contain rule name 'web-proxy'")
	}
}

func TestForwardRow_View_EmptyName(t *testing.T) {
	row := ForwardRow{
		Session: core.ForwardSession{
			ID: "s-empty",
			Rule: core.ForwardRule{
				Type:       core.Local,
				LocalPort:  3000,
				RemoteHost: "localhost",
				RemotePort: 80,
			},
			Status: core.Active,
		},
		HostName: "myhost",
		Width:    120,
	}

	out := row.View()
	if out == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(out, "3000") {
		t.Error("View() should contain local port '3000'")
	}
}

func TestForwardRow_View_LongNameTruncation(t *testing.T) {
	longName := "abcdefghijklmnopqrstuvwxyz1234" // 30 chars
	row := ForwardRow{
		Session: core.ForwardSession{
			ID: "s-long",
			Rule: core.ForwardRule{
				Name:       longName,
				Type:       core.Local,
				LocalPort:  9090,
				RemoteHost: "localhost",
				RemotePort: 80,
			},
			Status: core.Active,
		},
		Width: 120,
	}

	out := row.View()
	if strings.Contains(out, longName) {
		t.Error("View() should truncate long rule name")
	}
	if !strings.Contains(out, "abcdefghijklmnopqrs") {
		t.Error("View() should contain truncated prefix of long name")
	}
}

func TestForwardRow_View_NameWithoutHost(t *testing.T) {
	row := ForwardRow{
		Session: core.ForwardSession{
			ID: "s-nohost",
			Rule: core.ForwardRule{
				Name:       "db-tunnel",
				Type:       core.Local,
				LocalPort:  5432,
				RemoteHost: "dbhost",
				RemotePort: 5432,
			},
			Status: core.Active,
		},
		Width: 120,
	}

	out := row.View()
	if !strings.Contains(out, "db-tunnel") {
		t.Error("View() should contain rule name 'db-tunnel' even without host")
	}
}

func TestForwardRow_View_NarrowWidth(t *testing.T) {
	row := ForwardRow{
		Session: core.ForwardSession{
			ID: "s-narrow",
			Rule: core.ForwardRule{
				Name:       "abcdefghij", // 10 chars
				Type:       core.Local,
				LocalPort:  8080,
				RemoteHost: "localhost",
				RemotePort: 80,
			},
			Status: core.Active,
		},
		Width: 40, // limit = max(40/5, 6) = 8
	}

	out := row.View()
	if strings.Contains(out, "abcdefghij") {
		t.Error("View() should truncate name at narrow width")
	}
	if !strings.Contains(out, "abcdefg") {
		t.Error("View() should contain first 7 chars of truncated name")
	}
}

func TestForwardRow_View_Dynamic(t *testing.T) {
	row := ForwardRow{
		Session: core.ForwardSession{
			ID: "s2",
			Rule: core.ForwardRule{
				Type:      core.Dynamic,
				LocalPort: 1080,
			},
			Status: core.Active,
		},
		Width: 120,
	}

	out := row.View()
	if !strings.Contains(out, "SOCKS") {
		t.Error("View() should contain 'SOCKS' for dynamic forwarding")
	}
}

func TestForwardRow_View_WithTraffic(t *testing.T) {
	row := ForwardRow{
		Session: core.ForwardSession{
			ID: "s3",
			Rule: core.ForwardRule{
				Type:       core.Local,
				LocalPort:  3000,
				RemoteHost: "db",
				RemotePort: 5432,
			},
			Status:        core.Active,
			BytesSent:     2048,
			BytesReceived: 4096,
		},
		Width: 120,
	}

	out := row.View()
	if out == "" {
		t.Fatal("View() with traffic should produce non-empty output")
	}
}
