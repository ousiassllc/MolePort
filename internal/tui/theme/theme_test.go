package theme_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

func TestDefaultPresetID(t *testing.T) {
	got := theme.DefaultPresetID()
	if got != "dark-violet" {
		t.Errorf("DefaultPresetID() = %q, want %q", got, "dark-violet")
	}
}

func TestCurrent_DefaultIsDarkViolet(t *testing.T) {
	p := theme.Current()

	if p.Accent != lipgloss.Color("#7C3AED") {
		t.Errorf("Accent = %q, want %q", p.Accent, "#7C3AED")
	}
	if p.Text != lipgloss.Color("#E4E4E7") {
		t.Errorf("Text = %q, want %q", p.Text, "#E4E4E7")
	}
	if p.Error != lipgloss.Color("#EF4444") {
		t.Errorf("Error = %q, want %q", p.Error, "#EF4444")
	}
}

func TestApply_ChangesCurrentPalette(t *testing.T) {
	t.Cleanup(func() {
		theme.Apply(theme.DefaultPresetID())
	})

	theme.Apply("dark-blue")
	p := theme.Current()

	if p.Accent != lipgloss.Color("#3B82F6") {
		t.Errorf("after Apply(dark-blue): Accent = %q, want %q", p.Accent, "#3B82F6")
	}
	if p.AccentDim != lipgloss.Color("#2563EB") {
		t.Errorf("after Apply(dark-blue): AccentDim = %q, want %q", p.AccentDim, "#2563EB")
	}
}

func TestApply_UnknownIDKeepsCurrent(t *testing.T) {
	t.Cleanup(func() {
		theme.Apply(theme.DefaultPresetID())
	})

	before := theme.Current()
	theme.Apply("nonexistent")
	after := theme.Current()

	if before != after {
		t.Errorf("Apply(nonexistent) changed palette: before=%+v, after=%+v", before, after)
	}
}

func TestPresets_Returns10(t *testing.T) {
	all := theme.Presets()
	if len(all) != 10 {
		t.Fatalf("Presets() returned %d items, want 10", len(all))
	}

	// Dark×5 が先、Light×5 が後の順序を検証
	for i := 0; i < 5; i++ {
		if all[i].Base != "dark" {
			t.Errorf("Presets()[%d].Base = %q, want %q", i, all[i].Base, "dark")
		}
	}
	for i := 5; i < 10; i++ {
		if all[i].Base != "light" {
			t.Errorf("Presets()[%d].Base = %q, want %q", i, all[i].Base, "light")
		}
	}
}

func TestPresetsByBase(t *testing.T) {
	dark := theme.PresetsByBase("dark")
	if len(dark) != 5 {
		t.Errorf("PresetsByBase(dark) returned %d items, want 5", len(dark))
	}
	for _, p := range dark {
		if p.Base != "dark" {
			t.Errorf("PresetsByBase(dark) returned preset with Base=%q", p.Base)
		}
	}

	light := theme.PresetsByBase("light")
	if len(light) != 5 {
		t.Errorf("PresetsByBase(light) returned %d items, want 5", len(light))
	}
	for _, p := range light {
		if p.Base != "light" {
			t.Errorf("PresetsByBase(light) returned preset with Base=%q", p.Base)
		}
	}
}

func TestFindPreset_Found(t *testing.T) {
	p, ok := theme.FindPreset("dark-violet")
	if !ok {
		t.Fatal("FindPreset(dark-violet) returned ok=false")
	}
	if p.ID != "dark-violet" {
		t.Errorf("FindPreset(dark-violet).ID = %q, want %q", p.ID, "dark-violet")
	}
	if p.Base != "dark" {
		t.Errorf("FindPreset(dark-violet).Base = %q, want %q", p.Base, "dark")
	}
	if p.Accent != "violet" {
		t.Errorf("FindPreset(dark-violet).Accent = %q, want %q", p.Accent, "violet")
	}
	if p.Label != "Violet" {
		t.Errorf("FindPreset(dark-violet).Label = %q, want %q", p.Label, "Violet")
	}
}

func TestFindPreset_NotFound(t *testing.T) {
	_, ok := theme.FindPreset("nonexistent")
	if ok {
		t.Error("FindPreset(nonexistent) returned ok=true, want false")
	}
}

func TestPresetIDFromConfig(t *testing.T) {
	got := theme.PresetIDFromConfig("dark", "violet")
	if got != "dark-violet" {
		t.Errorf("PresetIDFromConfig(dark, violet) = %q, want %q", got, "dark-violet")
	}
}
