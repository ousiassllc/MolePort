package molecules

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInfoDialog_EnterDismisses(t *testing.T) {
	d := NewInfoDialog("update available")

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(InfoDismissedMsg); !ok {
		t.Fatalf("expected InfoDismissedMsg, got %T", msg)
	}
}

func TestInfoDialog_EscDismisses(t *testing.T) {
	d := NewInfoDialog("update available")

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(InfoDismissedMsg); !ok {
		t.Fatalf("expected InfoDismissedMsg, got %T", msg)
	}
}

func TestInfoDialog_OKeyDismisses(t *testing.T) {
	d := NewInfoDialog("update available")

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if cmd == nil {
		t.Fatal("o key should produce a command")
	}

	msg := cmd()
	if _, ok := msg.(InfoDismissedMsg); !ok {
		t.Fatalf("expected InfoDismissedMsg, got %T", msg)
	}
}

func TestInfoDialog_NonKeyMsgIgnored(t *testing.T) {
	d := NewInfoDialog("update available")

	_, cmd := d.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Error("non-key message should not produce a command")
	}
}

func TestInfoDialog_UnhandledKeyIgnored(t *testing.T) {
	d := NewInfoDialog("update available")

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Error("unhandled key should not produce a command")
	}
}
