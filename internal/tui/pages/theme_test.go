package pages_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/pages"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

func TestThemePage_EnterEmitsThemeSelectedMsg(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	p := pages.NewThemePage("dark-violet")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	selected, ok := msg.(tui.ThemeSelectedMsg)
	if !ok {
		t.Fatalf("expected ThemeSelectedMsg, got %T", msg)
	}
	if selected.PresetID != "dark-violet" {
		t.Errorf("PresetID = %q, want %q", selected.PresetID, "dark-violet")
	}
}

func TestThemePage_EscEmitsThemeCancelledMsg(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	p := pages.NewThemePage("dark-violet")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.ThemeCancelledMsg); !ok {
		t.Fatalf("expected ThemeCancelledMsg, got %T", msg)
	}
}

func TestThemePage_View_ContainsHelpText(t *testing.T) {
	p := pages.NewThemePage("dark-violet")
	p.SetSize(80, 24)
	view := p.View()

	for _, want := range []string{"Theme Select", "Enter", "Esc", "Cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() should contain %q", want)
		}
	}
}

func TestThemePage_SetSize(t *testing.T) {
	p := pages.NewThemePage("dark-violet")
	p.SetSize(100, 30)
	view := p.View()
	if view == "" {
		t.Error("View() should produce non-empty output after SetSize")
	}
}
