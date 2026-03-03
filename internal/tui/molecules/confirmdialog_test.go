package molecules

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmDialog_DefaultFocusIsNo(t *testing.T) {
	d := NewConfirmDialog("delete?")

	// Enter で決定 → デフォルトは No（focused=false）
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}

	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	if result.Confirmed {
		t.Error("default Enter should produce Confirmed=false")
	}
}

func TestConfirmDialog_ToggleFocusAndConfirm(t *testing.T) {
	d := NewConfirmDialog("delete?")

	// Tab で Yes にフォーカス切替
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})

	// Enter で決定 → Yes
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}

	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	if !result.Confirmed {
		t.Error("after toggle, Enter should produce Confirmed=true")
	}
}

func TestConfirmDialog_YKeyConfirms(t *testing.T) {
	d := NewConfirmDialog("delete?")

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("y key should produce a command")
	}

	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	if !result.Confirmed {
		t.Error("y key should produce Confirmed=true")
	}
}

func TestConfirmDialog_NKeyCancels(t *testing.T) {
	d := NewConfirmDialog("delete?")

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if cmd == nil {
		t.Fatal("n key should produce a command")
	}

	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	if result.Confirmed {
		t.Error("n key should produce Confirmed=false")
	}
}

func TestConfirmDialog_EscCancels(t *testing.T) {
	d := NewConfirmDialog("delete?")

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}

	msg := cmd()
	result, ok := msg.(ConfirmResultMsg)
	if !ok {
		t.Fatalf("expected ConfirmResultMsg, got %T", msg)
	}
	if result.Confirmed {
		t.Error("Esc should produce Confirmed=false")
	}
}

func TestConfirmDialog_NonKeyMsgIgnored(t *testing.T) {
	d := NewConfirmDialog("delete?")

	_, cmd := d.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Error("non-key message should not produce a command")
	}
}
