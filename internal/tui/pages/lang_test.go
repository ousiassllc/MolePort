package pages_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/pages"
)

func TestLangPage_EnterEmitsLangSelectedMsg(t *testing.T) {
	p := pages.NewLangPage("en")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	selected, ok := msg.(tui.LangSelectedMsg)
	if !ok {
		t.Fatalf("expected LangSelectedMsg, got %T", msg)
	}
	if selected.Lang != "en" {
		t.Errorf("Lang = %q, want %q", selected.Lang, "en")
	}
}

func TestLangPage_EscEmitsLangCancelledMsg(t *testing.T) {
	p := pages.NewLangPage("en")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(tui.LangCancelledMsg); !ok {
		t.Fatalf("expected LangCancelledMsg, got %T", msg)
	}
}

func TestLangPage_View_ContainsHelpText(t *testing.T) {
	p := pages.NewLangPage("en")
	p.SetSize(80, 24)
	view := p.View()

	for _, want := range []string{"Language", "Enter", "Esc", "English"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() should contain %q", want)
		}
	}
}

func TestLangPage_SetSize(t *testing.T) {
	p := pages.NewLangPage("en")
	p.SetSize(100, 30)
	view := p.View()
	if view == "" {
		t.Error("View() should produce non-empty output after SetSize")
	}
}

func TestLangPage_CursorNavigation(t *testing.T) {
	p := pages.NewLangPage("en")

	// en はインデックス 0 なので、↓ で ja に移動する
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := p.SelectedLang(); got != "ja" {
		t.Errorf("after Down, SelectedLang() = %q, want %q", got, "ja")
	}

	// ↑ で en に戻る
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := p.SelectedLang(); got != "en" {
		t.Errorf("after Up, SelectedLang() = %q, want %q", got, "en")
	}

	// 上端でさらに ↑ しても en のまま
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := p.SelectedLang(); got != "en" {
		t.Errorf("after Up at top, SelectedLang() = %q, want %q", got, "en")
	}
}

func TestLangPage_InitialCursorFromCurrentLang(t *testing.T) {
	p := pages.NewLangPage("ja")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	selected, ok := msg.(tui.LangSelectedMsg)
	if !ok {
		t.Fatalf("expected LangSelectedMsg, got %T", msg)
	}
	if selected.Lang != "ja" {
		t.Errorf("Lang = %q, want %q", selected.Lang, "ja")
	}
}
