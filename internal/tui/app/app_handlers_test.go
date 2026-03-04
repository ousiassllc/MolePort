package app

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

func updModel(m MainModel, msg tea.Msg) MainModel {
	result, _ := m.Update(msg)
	return result.(MainModel)
}

func keyMsg(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestHandleKeyMsg_Help(t *testing.T) {
	if !updModel(newTestModel("1.0.0"), keyMsg('?')).showHelpModal {
		t.Error("showHelpModal should be true")
	}
	m := newTestModel("1.0.0")
	m.showHelpModal = true
	if updModel(m, keyMsg('a')).showHelpModal {
		t.Error("showHelpModal should be false after any key")
	}
}

func TestHandleKeyMsg_Version(t *testing.T) {
	if got := updModel(newTestModel("1.2.3"), keyMsg('v')).dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1", got)
	}
}

func TestHandleKeyMsg_ThemeAndLang(t *testing.T) {
	cleanupTheme(t)
	cleanupLang(t)
	if updModel(newTestModel("1.0.0"), keyMsg('t')).currentPage != pageTheme {
		t.Error("'t' should open theme page")
	}
	if updModel(newTestModel("1.0.0"), keyMsg('l')).currentPage != pageLang {
		t.Error("'l' should open lang page")
	}
}

func TestHandleIPCMsg_HostsLoaded(t *testing.T) {
	u := updModel(newTestModel("1.0.0"), tui.HostsLoadedMsg{Err: fmt.Errorf("timeout")})
	if got := u.dashboard.LogLineCount(); got != 1 {
		t.Errorf("error: LogLineCount() = %d, want 1", got)
	}
	u = updModel(newTestModel("1.0.0"), tui.HostsLoadedMsg{Hosts: []core.SSHHost{{Name: "prod"}}})
	if len(u.hosts) != 1 || u.hosts[0].Name != "prod" {
		t.Errorf("hosts = %+v", u.hosts)
	}
}

func TestHandleIPCMsg_HostsReloaded(t *testing.T) {
	u := updModel(newTestModel("1.0.0"), tui.HostsReloadedMsg{Err: fmt.Errorf("err")})
	if got := u.dashboard.LogLineCount(); got != 1 {
		t.Errorf("error: LogLineCount() = %d, want 1", got)
	}
	u = updModel(newTestModel("1.0.0"), tui.HostsReloadedMsg{Hosts: []core.SSHHost{{Name: "s"}}})
	if len(u.hosts) != 1 {
		t.Errorf("hosts len = %d, want 1", len(u.hosts))
	}
}

func TestHandleIPCMsg_SessionsLoaded(t *testing.T) {
	u := updModel(newTestModel("1.0.0"), sessionsLoadedMsg{
		Sessions: []core.ForwardSession{{ID: "s1", Rule: core.ForwardRule{Name: "web"}}},
	})
	if len(u.sessions) != 1 || u.sessions[0].ID != "s1" {
		t.Errorf("sessions = %+v", u.sessions)
	}
}

func TestHandleIPCMsg_MetricsTick_ReturnsCmd(t *testing.T) {
	if _, cmd := newTestModel("1.0.0").Update(tui.MetricsTickMsg{}); cmd == nil {
		t.Error("MetricsTickMsg should return commands")
	}
}

func TestHandleForwardMsg_LogOutput(t *testing.T) {
	if got := updModel(newTestModel("1.0.0"), tui.LogOutputMsg{Text: "ok"}).dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1", got)
	}
	m := newTestModel("1.0.0")
	m.restarting = true
	if got := updModel(m, tui.LogOutputMsg{Text: "x"}).dashboard.LogLineCount(); got != 0 {
		t.Errorf("restarting: LogLineCount() = %d, want 0", got)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	u := updModel(newTestModel("1.0.0"), tea.WindowSizeMsg{Width: 120, Height: 40})
	if u.width != 120 || u.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", u.width, u.height)
	}
}

func TestView_HelpModal(t *testing.T) {
	m := newTestModel("1.0.0")
	m.width, m.height = 80, 24
	m.showHelpModal = true
	if m.View() == "" {
		t.Error("should render help overlay")
	}
}
