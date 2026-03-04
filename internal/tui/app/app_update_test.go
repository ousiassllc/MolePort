package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

func TestUncoveredBranches(t *testing.T) {
	if r, cmd := newTestModel("1").Update(tea.KeyMsg{Type: tea.KeyCtrlC}); !r.(MainModel).quitting || cmd == nil {
		t.Error("ctrl+c")
	}
	m := newTestModel("1")
	m.showUpdateNotify, m.updateNotifyDialog = true, molecules.NewInfoDialog("t")
	if _, _, ok := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}); !ok {
		t.Error("updateNotify")
	}
	m2 := newTestModel("1")
	m2.showVersionConfirm, m2.versionConfirm = true, molecules.NewConfirmDialog("t")
	if _, _, ok := m2.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}); !ok {
		t.Error("versionConfirm")
	}
	for _, p := range []string{pageTheme, pageLang} {
		m3 := newTestModel("1")
		m3.currentPage = p
		if _, _, ok := m3.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}); !ok {
			t.Errorf("page %s", p)
		}
		m3.width, m3.height = 80, 24
		if m3.View() == "" {
			t.Errorf("view %s", p)
		}
	}
	if _, c, ok := newTestModel("1").handleForwardMsg(tui.ForwardToggleMsg{RuleName: "w"}); !ok || c == nil {
		t.Error("toggle")
	}
	if _, c, ok := newTestModel("1").handleForwardMsg(tui.ForwardDeleteRequestMsg{RuleName: "w"}); !ok || c == nil {
		t.Error("delete")
	}
	if r, c, ok := newTestModel("1").handleUIMsg(tui.QuitRequestMsg{}); !ok || c == nil || !r.quitting {
		t.Error("quit")
	}
	if _, c, ok := newTestModel("1").handleUIMsg(molecules.InfoDismissedMsg{}); !ok || c != nil {
		t.Error("infoDismissed")
	}
	// ForwardDeleteConfirmedMsg
	if _, c, ok := newTestModel("1").handleForwardMsg(tui.ForwardDeleteConfirmedMsg{RuleName: "w"}); !ok || c == nil {
		t.Error("deleteConfirmed")
	}
	// handleSystemMsg fallthrough (unhandled msg type)
	if _, _, ok := newTestModel("1").handleSystemMsg("unknown"); ok {
		t.Error("unknown msg should not be handled")
	}
	// handleForwardMsg fallthrough
	if _, _, ok := newTestModel("1").handleForwardMsg("unknown"); ok {
		t.Error("unknown forward msg should not be handled")
	}
	// handleUIMsg fallthrough
	if _, _, ok := newTestModel("1").handleUIMsg("unknown"); ok {
		t.Error("unknown ui msg should not be handled")
	}
	// handleIPCMsg fallthrough
	if _, _, ok := newTestModel("1").handleIPCMsg("unknown"); ok {
		t.Error("unknown ipc msg should not be handled")
	}
	// SetDaemonManager
	m4 := newTestModel("1")
	m4.SetDaemonManager(nil)
	// handleCredentialSubmit with nil channel
	if r, c := newTestModel("1").handleCredentialSubmit(tui.CredentialSubmitMsg{}); r.(MainModel).quitting || c != nil {
		t.Error("credSubmit nil ch")
	}
}
