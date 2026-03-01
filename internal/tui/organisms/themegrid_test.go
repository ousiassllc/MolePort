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

func specialKeyMsg(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func TestNewThemeGrid_DefaultPosition(t *testing.T) {
	g := organisms.NewThemeGrid("dark-violet")
	if got := g.SelectedPresetID(); got != "dark-violet" {
		t.Errorf("SelectedPresetID() = %q, want %q", got, "dark-violet")
	}
}

func TestNewThemeGrid_LightPreset(t *testing.T) {
	g := organisms.NewThemeGrid("light-cyan")
	if got := g.SelectedPresetID(); got != "light-cyan" {
		t.Errorf("SelectedPresetID() = %q, want %q", got, "light-cyan")
	}
}

func TestNewThemeGrid_UnknownPreset(t *testing.T) {
	g := organisms.NewThemeGrid("nonexistent-theme")
	if got := g.SelectedPresetID(); got != "dark-violet" {
		t.Errorf("SelectedPresetID() = %q, want %q (default)", got, "dark-violet")
	}
}

func TestThemeGrid_UpDown(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	g := organisms.NewThemeGrid("dark-violet")
	initial := g.SelectedPresetID()

	// Down: 2番目のプリセットに移動
	g, _ = g.Update(specialKeyMsg(tea.KeyDown))
	after := g.SelectedPresetID()
	if after == initial {
		t.Error("Down should change SelectedPresetID")
	}

	// Up: 元に戻る
	g, _ = g.Update(specialKeyMsg(tea.KeyUp))
	if got := g.SelectedPresetID(); got != initial {
		t.Errorf("Up should restore SelectedPresetID: got %q, want %q", got, initial)
	}

	// 上端クランプ: もう一度 Up しても変わらない
	g, _ = g.Update(specialKeyMsg(tea.KeyUp))
	if got := g.SelectedPresetID(); got != initial {
		t.Errorf("Up at top should clamp: got %q, want %q", got, initial)
	}

	// 下端クランプ: 十分に Down しても範囲外にならない
	for i := 0; i < 20; i++ {
		g, _ = g.Update(specialKeyMsg(tea.KeyDown))
	}
	dark := theme.PresetsByBase("dark")
	last := dark[len(dark)-1].ID
	if got := g.SelectedPresetID(); got != last {
		t.Errorf("Down at bottom should clamp to last: got %q, want %q", got, last)
	}
}

func TestThemeGrid_LeftRight(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	g := organisms.NewThemeGrid("dark-violet")

	// Right: light カラムに移動
	g, _ = g.Update(specialKeyMsg(tea.KeyRight))
	got := g.SelectedPresetID()
	if !strings.HasPrefix(got, "light-") {
		t.Errorf("Right should move to light column: got %q", got)
	}

	// Left: dark カラムに戻る
	g, _ = g.Update(specialKeyMsg(tea.KeyLeft))
	got = g.SelectedPresetID()
	if !strings.HasPrefix(got, "dark-") {
		t.Errorf("Left should move back to dark column: got %q", got)
	}

	// Left クランプ: もう一度 Left しても dark のまま
	g, _ = g.Update(specialKeyMsg(tea.KeyLeft))
	got = g.SelectedPresetID()
	if !strings.HasPrefix(got, "dark-") {
		t.Errorf("Left at leftmost should clamp: got %q", got)
	}
}

func TestThemeGrid_ApplyOnMove(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	g := organisms.NewThemeGrid("dark-violet")

	// Down で移動 → テーマが自動適用される
	g, _ = g.Update(specialKeyMsg(tea.KeyDown))

	selectedID := g.SelectedPresetID()
	p, ok := theme.FindPreset(selectedID)
	if !ok {
		t.Fatalf("FindPreset(%q) not found", selectedID)
	}

	cur := theme.Current()
	if cur.Accent != p.Palette.Accent {
		t.Errorf("Current().Accent = %q, want %q (from preset %q)",
			cur.Accent, p.Palette.Accent, selectedID)
	}
}

func TestThemeGrid_SelectedPresetID(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	dark := theme.PresetsByBase("dark")
	light := theme.PresetsByBase("light")

	g := organisms.NewThemeGrid("dark-violet")

	// Dark カラムの各位置を確認
	for i, want := range dark {
		if i > 0 {
			g, _ = g.Update(specialKeyMsg(tea.KeyDown))
		}
		if got := g.SelectedPresetID(); got != want.ID {
			t.Errorf("dark[%d]: SelectedPresetID() = %q, want %q", i, got, want.ID)
		}
	}

	// Light カラムに移動して先頭を確認
	g = organisms.NewThemeGrid("dark-violet")
	g, _ = g.Update(specialKeyMsg(tea.KeyRight))
	if got := g.SelectedPresetID(); got != light[0].ID {
		t.Errorf("light[0]: SelectedPresetID() = %q, want %q", got, light[0].ID)
	}
}

func TestThemeGrid_View_ContainsLabels(t *testing.T) {
	g := organisms.NewThemeGrid("dark-violet")
	g.SetSize(60, 20)
	view := g.View()

	for _, label := range []string{"Dark", "Light", "Violet"} {
		if !strings.Contains(view, label) {
			t.Errorf("View() should contain %q", label)
		}
	}

	// スウォッチ記号が含まれることを確認
	if !strings.Contains(view, "●") {
		t.Error("View() should contain color swatch ●")
	}
}

func TestThemeGrid_View_SelectedIndicator(t *testing.T) {
	g := organisms.NewThemeGrid("dark-violet")
	g.SetSize(60, 20)
	view := g.View()

	if !strings.Contains(view, ">") {
		t.Error("View() should contain > for selected row")
	}
}

func TestThemeGrid_SetSize(t *testing.T) {
	g := organisms.NewThemeGrid("dark-violet")
	g.SetSize(80, 30)

	// SetSize 後に View が正常動作するか確認
	view := g.View()
	if view == "" {
		t.Error("View() should produce non-empty output after SetSize")
	}

	// 幅が反映されているか (columnWidth = (80-2)/2 = 39)
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Fatal("View() produced no lines")
	}
	totalWidth := lipgloss.Width(view)
	if totalWidth == 0 {
		t.Error("View() has zero width")
	}
}

func TestThemeGrid_HJKLKeys(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	g := organisms.NewThemeGrid("dark-violet")

	// j = Down
	g, _ = g.Update(keyMsg("j"))
	if got := g.SelectedPresetID(); got == "dark-violet" {
		t.Error("j key should move down from first preset")
	}

	// k = Up
	g, _ = g.Update(keyMsg("k"))
	if got := g.SelectedPresetID(); got != "dark-violet" {
		t.Errorf("k key should move up: got %q", got)
	}

	// l = Right
	g, _ = g.Update(keyMsg("l"))
	if got := g.SelectedPresetID(); !strings.HasPrefix(got, "light-") {
		t.Errorf("l key should move to light column: got %q", got)
	}

	// h = Left
	g, _ = g.Update(keyMsg("h"))
	if got := g.SelectedPresetID(); !strings.HasPrefix(got, "dark-") {
		t.Errorf("h key should move to dark column: got %q", got)
	}
}
