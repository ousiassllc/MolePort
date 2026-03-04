package pages

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

func newTestDashboard() DashboardPage {
	d := NewDashboardPage("0.1.0")
	d.SetSize(80, 24)
	return d
}

func TestDashboardSlashKeyFocusesSetupPanel(t *testing.T) {
	d := newTestDashboard()
	if d.FocusedPane() != tui.PaneSetup {
		t.Fatalf("initial focus = %v, want PaneSetup", d.FocusedPane())
	}
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyTab})
	if d.FocusedPane() != tui.PaneForwards {
		t.Fatalf("after Tab: focus = %v, want PaneForwards", d.FocusedPane())
	}
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if d.FocusedPane() != tui.PaneSetup {
		t.Errorf("after /: focus = %v, want PaneSetup", d.FocusedPane())
	}
}

func TestDashboardSlashKeyNoopWhenAlreadySetup(t *testing.T) {
	d := newTestDashboard()
	if d.FocusedPane() != tui.PaneSetup {
		t.Fatalf("initial focus = %v, want PaneSetup", d.FocusedPane())
	}
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if d.FocusedPane() != tui.PaneSetup {
		t.Errorf("after / when already setup: focus = %v, want PaneSetup", d.FocusedPane())
	}
}

func TestDashboardSlashKeyIgnoredDuringInput(t *testing.T) {
	d := newTestDashboard()
	hosts := []core.SSHHost{{Name: "test-host", State: core.Disconnected}}
	d.SetHosts(hosts)
	if d.IsInputActive() {
		t.Fatalf("expected IsInputActive() = false in idle state")
	}
}
