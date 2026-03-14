package organisms_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui/organisms"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}
func specialKeyMsg(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func TestNewThemeGrid_Presets(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"dark-violet", "dark-violet"},
		{"light-cyan", "light-cyan"},
		{"nonexistent-theme", "dark-violet"},
	} {
		if got := organisms.NewThemeGrid(tt.in).SelectedPresetID(); got != tt.want {
			t.Errorf("NewThemeGrid(%q).SelectedPresetID()=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestThemeGrid_UpDown(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	g := organisms.NewThemeGrid("dark-violet")
	initial := g.SelectedPresetID()
	g, _ = g.Update(specialKeyMsg(tea.KeyDown))
	if g.SelectedPresetID() == initial {
		t.Error("Down should change SelectedPresetID")
	}
	g, _ = g.Update(specialKeyMsg(tea.KeyUp))
	if got := g.SelectedPresetID(); got != initial {
		t.Errorf("Up should restore: got %q want %q", got, initial)
	}
	g, _ = g.Update(specialKeyMsg(tea.KeyUp)) // clamp at top
	if got := g.SelectedPresetID(); got != initial {
		t.Errorf("Up at top should clamp: got %q", got)
	}
	for range 20 {
		g, _ = g.Update(specialKeyMsg(tea.KeyDown))
	}
	dark := theme.PresetsByBase("dark")
	if got := g.SelectedPresetID(); got != dark[len(dark)-1].ID {
		t.Errorf("Down at bottom should clamp to last: got %q", got)
	}
}

func TestThemeGrid_LeftRight(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	g := organisms.NewThemeGrid("dark-violet")
	g, _ = g.Update(specialKeyMsg(tea.KeyRight))
	if got := g.SelectedPresetID(); !strings.HasPrefix(got, "light-") {
		t.Errorf("Right should move to light: got %q", got)
	}
	g, _ = g.Update(specialKeyMsg(tea.KeyLeft))
	if got := g.SelectedPresetID(); !strings.HasPrefix(got, "dark-") {
		t.Errorf("Left should move to dark: got %q", got)
	}
	g, _ = g.Update(specialKeyMsg(tea.KeyLeft)) // clamp
	if got := g.SelectedPresetID(); !strings.HasPrefix(got, "dark-") {
		t.Errorf("Left at leftmost should clamp: got %q", got)
	}
}

func TestThemeGrid_ApplyOnMove(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	g := organisms.NewThemeGrid("dark-violet")
	g, _ = g.Update(specialKeyMsg(tea.KeyDown))
	p, ok := theme.FindPreset(g.SelectedPresetID())
	if !ok {
		t.Fatalf("FindPreset(%q) not found", g.SelectedPresetID())
	}
	if cur := theme.Current(); cur.Accent != p.Palette.Accent {
		t.Errorf("Accent=%q want %q", cur.Accent, p.Palette.Accent)
	}
}

func TestThemeGrid_SelectedPresetID_Traversal(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	dark, light := theme.PresetsByBase("dark"), theme.PresetsByBase("light")
	g := organisms.NewThemeGrid("dark-violet")
	for i, want := range dark {
		if i > 0 {
			g, _ = g.Update(specialKeyMsg(tea.KeyDown))
		}
		if got := g.SelectedPresetID(); got != want.ID {
			t.Errorf("dark[%d]=%q want %q", i, got, want.ID)
		}
	}
	g = organisms.NewThemeGrid("dark-violet")
	g, _ = g.Update(specialKeyMsg(tea.KeyRight))
	if got := g.SelectedPresetID(); got != light[0].ID {
		t.Errorf("light[0]=%q want %q", got, light[0].ID)
	}
}

func TestThemeGrid_View(t *testing.T) {
	g := organisms.NewThemeGrid("dark-violet")
	g.SetSize(60, 20)
	view := g.View()
	for _, label := range []string{"Dark", "Light", "Violet", "●", ">"} {
		if !strings.Contains(view, label) {
			t.Errorf("View() should contain %q", label)
		}
	}
}

func TestThemeGrid_SetSize(t *testing.T) {
	g := organisms.NewThemeGrid("dark-violet")
	g.SetSize(80, 30)
	view := g.View()
	if view == "" {
		t.Error("View() should be non-empty after SetSize")
	}
	if lipgloss.Width(view) == 0 {
		t.Error("View() has zero width")
	}
}

func TestThemeGrid_HJKLKeys(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	g := organisms.NewThemeGrid("dark-violet")
	g, _ = g.Update(keyMsg("j"))
	if g.SelectedPresetID() == "dark-violet" {
		t.Error("j should move down")
	}
	g, _ = g.Update(keyMsg("k"))
	if g.SelectedPresetID() != "dark-violet" {
		t.Error("k should move up")
	}
	g, _ = g.Update(keyMsg("l"))
	if !strings.HasPrefix(g.SelectedPresetID(), "light-") {
		t.Error("l should move to light")
	}
	g, _ = g.Update(keyMsg("h"))
	if !strings.HasPrefix(g.SelectedPresetID(), "dark-") {
		t.Error("h should move to dark")
	}
}
