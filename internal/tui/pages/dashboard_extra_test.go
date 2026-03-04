package pages

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

func TestNewDashboardPage(t *testing.T) {
	d := NewDashboardPage("1.2.3")
	if d.FocusedPane() != tui.PaneSetup {
		t.Errorf("initial focus = %v, want PaneSetup", d.FocusedPane())
	}
	if d.version != "1.2.3" {
		t.Errorf("version = %q, want %q", d.version, "1.2.3")
	}
	if d.LogLineCount() != 0 {
		t.Errorf("initial LogLineCount = %d, want 0", d.LogLineCount())
	}
}

func TestDashboardInit(t *testing.T) {
	if cmd := NewDashboardPage("0.1.0").Init(); cmd != nil {
		t.Errorf("Init() should return nil")
	}
}

func TestDashboardView(t *testing.T) {
	t.Run("before_set_size", func(t *testing.T) {
		if v := NewDashboardPage("0.1.0").View(); v != "Loading..." {
			t.Errorf("View() = %q, want Loading...", v)
		}
	})
	t.Run("after_set_size", func(t *testing.T) {
		v := newTestDashboard().View()
		if v == "" || v == "Loading..." {
			t.Error("View() should produce output after SetSize")
		}
		if !strings.Contains(v, "MolePort") {
			t.Error("View() should contain MolePort header")
		}
	})
	t.Run("contains_version", func(t *testing.T) {
		d := NewDashboardPage("9.8.7")
		d.SetSize(120, 30)
		if !strings.Contains(d.View(), "9.8.7") {
			t.Error("View() should contain version string")
		}
	})
}

func TestDashboardRenderHeader(t *testing.T) {
	for _, tt := range []struct {
		name  string
		width int
		wants []string
	}{
		{"narrow", 5, []string{"MolePort"}},
		{"wide", 120, []string{"MolePort", "0.1.0"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDashboardPage("0.1.0")
			d.SetSize(tt.width, 24)
			h := d.renderHeader()
			for _, w := range tt.wants {
				if !strings.Contains(h, w) {
					t.Errorf("renderHeader() missing %q", w)
				}
			}
		})
	}
}

func TestSetForwardSessions(t *testing.T) {
	d := newTestDashboard()
	d.SetForwardSessions([]core.ForwardSession{{ID: "s1", Status: core.Active}, {ID: "s2", Status: core.Stopped}})
	if v := d.View(); v == "" || v == "Loading..." {
		t.Error("View() should render after SetForwardSessions")
	}
}

func TestAppendLogAndLogLineCount(t *testing.T) {
	d := newTestDashboard()
	if d.LogLineCount() != 0 {
		t.Fatalf("initial LogLineCount = %d", d.LogLineCount())
	}
	d.AppendLog("line 1")
	d.AppendLog("line 2")
	d.AppendLog("line 3")
	if d.LogLineCount() != 3 {
		t.Errorf("LogLineCount = %d, want 3", d.LogLineCount())
	}
}

func TestUpdateHostState(t *testing.T) {
	d := newTestDashboard()
	d.SetHosts([]core.SSHHost{{Name: "host-a", State: core.Disconnected}})
	d.UpdateHostState("host-a", core.Connected)
	if v := d.View(); v == "" || v == "Loading..." {
		t.Error("View() should render after UpdateHostState")
	}
}

func TestSetVersionWarning(t *testing.T) {
	d := newTestDashboard()
	for _, on := range []bool{true, false} {
		d.SetVersionWarning(on)
		if d.View() == "" {
			t.Errorf("View() empty after SetVersionWarning(%v)", on)
		}
	}
}

func TestShowPasswordInput(t *testing.T) {
	d := newTestDashboard()
	if d.IsInputActive() {
		t.Fatal("IsInputActive() should be false initially")
	}
	if cmd := d.ShowPasswordInput("Enter password:"); cmd == nil {
		t.Fatal("ShowPasswordInput should return non-nil cmd")
	}
	if v := d.View(); v == "" || v == "Loading..." {
		t.Error("View() should render after ShowPasswordInput")
	}
}

func TestCycleFocusRotation(t *testing.T) {
	d := newTestDashboard()
	expects := []tui.FocusPane{tui.PaneForwards, tui.PaneSetup, tui.PaneForwards}
	for i, want := range expects {
		d, _ = d.Update(tea.KeyMsg{Type: tea.KeyTab})
		if d.FocusedPane() != want {
			t.Errorf("Tab #%d: focus = %v, want %v", i+1, d.FocusedPane(), want)
		}
	}
}

func TestHandleSSHEvent(t *testing.T) {
	d := newTestDashboard()
	d.SetHosts([]core.SSHHost{{Name: "host-a", State: core.Disconnected}})
	for _, et := range []core.SSHEventType{
		core.SSHEventConnected, core.SSHEventDisconnected, core.SSHEventReconnecting, core.SSHEventPendingAuth, core.SSHEventError,
	} {
		d, _ = d.Update(tui.SSHEventMsg{Event: core.SSHEvent{Type: et, HostName: "host-a"}})
	}
	if v := d.View(); v == "" || v == "Loading..." {
		t.Error("View() should render after SSHEvent processing")
	}
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	d := NewDashboardPage("0.1.0")
	d, cmd := d.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
	if d.View() == "Loading..." {
		t.Error("View() should not be Loading... after WindowSizeMsg")
	}
}

func TestUpdateLogOutputMsg(t *testing.T) {
	d := newTestDashboard()
	d, _ = d.Update(tui.LogOutputMsg{Text: "hello log"})
	if d.LogLineCount() != 1 {
		t.Errorf("LogLineCount = %d, want 1", d.LogLineCount())
	}
}

func TestUpdateKeyForward(t *testing.T) {
	t.Run("forward_panel", func(t *testing.T) {
		d := newTestDashboard()
		d, _ = d.Update(tea.KeyMsg{Type: tea.KeyTab})
		d, _ = d.Update(tea.KeyMsg{Type: tea.KeyDown})
		if d.FocusedPane() != tui.PaneForwards {
			t.Errorf("focus = %v, want PaneForwards", d.FocusedPane())
		}
	})
	t.Run("setup_panel", func(t *testing.T) {
		d := newTestDashboard()
		d, _ = d.Update(tea.KeyMsg{Type: tea.KeyDown})
		if d.FocusedPane() != tui.PaneSetup {
			t.Errorf("focus = %v, want PaneSetup", d.FocusedPane())
		}
	})
}

func TestUpdateSizes(t *testing.T) {
	t.Run("small", func(t *testing.T) {
		d := NewDashboardPage("0.1.0")
		d.SetSize(40, 10)
		if v := d.View(); v == "" || v == "Loading..." {
			t.Error("View() should render with small screen")
		}
	})
	t.Run("zero", func(t *testing.T) {
		d := NewDashboardPage("0.1.0")
		d.SetSize(0, 0)
		if d.View() != "Loading..." {
			t.Error("View() with zero size should be Loading...")
		}
	})
}

func TestPasswordInputCapturesKeys(t *testing.T) {
	d := newTestDashboard()
	d.ShowPasswordInput("Password:")
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	focus := d.FocusedPane()
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyTab})
	if d.FocusedPane() != focus {
		t.Error("Tab should not change focus during password input")
	}
}
