package organisms

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

func makeSessions(names ...string) []core.ForwardSession {
	var ss []core.ForwardSession
	for _, n := range names {
		ss = append(ss, core.ForwardSession{
			Rule:   core.ForwardRule{Name: n, Host: "host1", LocalPort: 8080},
			Status: core.Active,
		})
	}
	return ss
}

func TestNewForwardPanel(t *testing.T) {
	p := NewForwardPanel()
	if len(p.sessions) != 0 || p.cursor != 0 || p.focused {
		t.Errorf("NewForwardPanel: sessions=%d cursor=%d focused=%v", len(p.sessions), p.cursor, p.focused)
	}
}

func TestForwardPanel_SetFocused(t *testing.T) {
	p := NewForwardPanel()
	p.SetFocused(true)
	if !p.focused {
		t.Error("SetFocused(true) failed")
	}
	p.SetFocused(false)
	if p.focused {
		t.Error("SetFocused(false) failed")
	}
}

func TestForwardPanel_SetSessions_Roundtrip(t *testing.T) {
	p := NewForwardPanel()
	p.SetSessions(makeSessions("a", "b", "c"))
	if got := p.Sessions(); len(got) != 3 || got[0].Rule.Name != "a" {
		t.Errorf("Sessions roundtrip failed: len=%d", len(got))
	}
}

func TestForwardPanel_SetSessions_AdjustsCursor(t *testing.T) {
	p := NewForwardPanel()
	p.SetSessions(makeSessions("a", "b", "c"))
	p.cursor = 2
	p.SetSessions(makeSessions("a"))
	if p.cursor != 0 {
		t.Errorf("cursor after shrink=%d want 0", p.cursor)
	}
	p.SetSessions(nil)
	if p.cursor != 0 {
		t.Errorf("cursor after empty=%d want 0", p.cursor)
	}
}

func TestForwardPanel_Update_NotFocused(t *testing.T) {
	p := NewForwardPanel()
	p.SetSessions(makeSessions("a", "b"))
	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil || p2.cursor != 0 {
		t.Error("Update when not focused should be noop")
	}
}

func TestForwardPanel_Update_UpDown(t *testing.T) {
	p := NewForwardPanel()
	p.SetFocused(true)
	p.SetSessions(makeSessions("a", "b", "c"))

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.cursor != 1 {
		t.Errorf("after Down: cursor=%d want 1", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown}) // at end
	if p.cursor != 2 {
		t.Errorf("Down at end: cursor=%d want 2", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.cursor != 1 {
		t.Errorf("after Up: cursor=%d want 1", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp}) // at top
	if p.cursor != 0 {
		t.Errorf("Up at top: cursor=%d want 0", p.cursor)
	}
}

func TestForwardPanel_Update_Enter(t *testing.T) {
	p := NewForwardPanel()
	p.SetFocused(true)
	p.SetSessions(makeSessions("my-rule"))
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a cmd")
	}
	toggle, ok := cmd().(tui.ForwardToggleMsg)
	if !ok {
		t.Fatalf("expected ForwardToggleMsg, got %T", cmd())
	}
	if toggle.RuleName != "my-rule" {
		t.Errorf("RuleName=%q want my-rule", toggle.RuleName)
	}
}

func TestForwardPanel_Update_Delete(t *testing.T) {
	p := NewForwardPanel()
	p.SetFocused(true)
	p.SetSessions(makeSessions("del-rule"))
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("Delete key should produce a cmd")
	}
	del, ok := cmd().(tui.ForwardDeleteRequestMsg)
	if !ok {
		t.Fatalf("expected ForwardDeleteRequestMsg, got %T", cmd())
	}
	if del.RuleName != "del-rule" {
		t.Errorf("RuleName=%q want del-rule", del.RuleName)
	}
}

func TestForwardPanel_Update_NonKeyMsg(t *testing.T) {
	p := NewForwardPanel()
	p.SetFocused(true)
	_, cmd := p.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Error("non-key msg should return nil cmd")
	}
}

func TestForwardPanel_Update_Enter_NoSessions(t *testing.T) {
	p := NewForwardPanel()
	p.SetFocused(true)
	if _, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("Enter with no sessions should return nil cmd")
	}
}

func TestForwardPanel_View(t *testing.T) {
	p := NewForwardPanel()
	p.SetSize(40, 10)
	if v := p.View(); v == "" || !strings.Contains(v, "0") {
		t.Error("empty View should be non-empty and contain '0'")
	}
	p.SetFocused(true)
	p.SetSessions(makeSessions("web", "api"))
	p.SetSize(60, 10)
	if p.View() == "" {
		t.Error("View with sessions should be non-empty")
	}
}
