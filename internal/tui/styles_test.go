package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

func TestRenderWithBorderTitle_EmptyTitle(t *testing.T) {
	style := UnfocusedBorder()
	rendered := RenderWithBorderTitle(style, 20, 3, "", "hello")
	plain := style.Width(20).Height(3).Render("hello")
	if rendered != plain {
		t.Errorf("empty title should return same output as plain render")
	}
}

func TestRenderWithBorderTitle_HasTitle(t *testing.T) {
	style := FocusedBorder()
	rendered := RenderWithBorderTitle(style, 30, 3, "My Title", "content")

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	topLine := lines[0]
	if !strings.Contains(topLine, "My Title") {
		t.Errorf("top border line should contain title, got %q", topLine)
	}

	b := lipgloss.RoundedBorder()
	if !strings.Contains(topLine, b.TopLeft) {
		t.Errorf("top border line should start with TopLeft corner")
	}
	if !strings.Contains(topLine, b.TopRight) {
		t.Errorf("top border line should end with TopRight corner")
	}
}

func TestRenderWithBorderTitle_PreservesWidth(t *testing.T) {
	style := UnfocusedBorder()
	width := 40

	withTitle := RenderWithBorderTitle(style, width, 3, "Test", "body")
	without := style.Width(width).Height(3).Render("body")

	wWith := lipgloss.Width(strings.Split(withTitle, "\n")[0])
	wWithout := lipgloss.Width(strings.Split(without, "\n")[0])

	if wWith != wWithout {
		t.Errorf("top border width with title (%d) should equal without title (%d)", wWith, wWithout)
	}
}

func TestRenderWithBorderTitle_LongTitle(t *testing.T) {
	style := UnfocusedBorder()
	longTitle := strings.Repeat("A", 100)
	// Should not panic even if title exceeds border width
	rendered := RenderWithBorderTitle(style, 20, 3, longTitle, "body")
	if rendered == "" {
		t.Error("should produce non-empty output even with long title")
	}
}

func TestStyles_ReflectThemeChange(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	s1 := ActiveStyle()
	theme.Apply("dark-blue")
	s2 := ActiveStyle()
	if s1.GetForeground() == s2.GetForeground() {
		t.Error("ActiveStyle should change after theme.Apply")
	}
}

// --- カラー関数テスト ---

func TestColorFunctions_ReturnNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		fn   func() lipgloss.Color
	}{
		{"AccentColor", AccentColor},
		{"TextColor", TextColor},
		{"MutedColor", MutedColor},
		{"ErrorColor", ErrorColor},
		{"WarningColor", WarningColor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.fn()
			if string(c) == "" {
				t.Errorf("%s() returned empty color", tt.name)
			}
		})
	}
}

func TestColorFunctions_MatchTheme(t *testing.T) {
	p := theme.Current()
	if AccentColor() != p.Accent {
		t.Error("AccentColor should match theme.Current().Accent")
	}
	if TextColor() != p.Text {
		t.Error("TextColor should match theme.Current().Text")
	}
	if MutedColor() != p.Muted {
		t.Error("MutedColor should match theme.Current().Muted")
	}
	if ErrorColor() != p.Error {
		t.Error("ErrorColor should match theme.Current().Error")
	}
	if WarningColor() != p.Warning {
		t.Error("WarningColor should match theme.Current().Warning")
	}
}

// --- スタイル関数テスト ---

func TestStyleFunctions_ReturnValidStyles(t *testing.T) {
	tests := []struct {
		name string
		fn   func() lipgloss.Style
	}{
		{"TitleStyle", TitleStyle},
		{"MutedStyle", MutedStyle},
		{"SelectedStyle", SelectedStyle},
		{"TextStyle", TextStyle},
		{"ActiveStyle", ActiveStyle},
		{"StoppedStyle", StoppedStyle},
		{"ErrorStyle", ErrorStyle},
		{"ReconnectingStyle", ReconnectingStyle},
		{"WarningStyle", WarningStyle},
		{"KeyStyle", KeyStyle},
		{"DescStyle", DescStyle},
		{"DividerStyle", DividerStyle},
		{"HeaderStyle", HeaderStyle},
		{"StatusBarStyle", StatusBarStyle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fn()
			// スタイルが何らかの文字列をレンダリングできることを確認
			rendered := s.Render("test")
			if rendered == "" {
				t.Errorf("%s().Render(\"test\") returned empty string", tt.name)
			}
		})
	}
}

func TestTitleStyle_IsBold(t *testing.T) {
	s := TitleStyle()
	if !s.GetBold() {
		t.Error("TitleStyle should be bold")
	}
}

func TestWarningStyle_IsBold(t *testing.T) {
	s := WarningStyle()
	if !s.GetBold() {
		t.Error("WarningStyle should be bold")
	}
}

func TestKeyStyle_IsBold(t *testing.T) {
	s := KeyStyle()
	if !s.GetBold() {
		t.Error("KeyStyle should be bold")
	}
}

func TestHeaderStyle_IsBold(t *testing.T) {
	s := HeaderStyle()
	if !s.GetBold() {
		t.Error("HeaderStyle should be bold")
	}
}

func TestSelectedStyle_IsBold(t *testing.T) {
	s := SelectedStyle()
	if !s.GetBold() {
		t.Error("SelectedStyle should be bold")
	}
}

func TestFocusedBorder_HasBorder(t *testing.T) {
	s := FocusedBorder()
	if !s.GetBorderTop() {
		t.Error("FocusedBorder should have a top border")
	}
}

func TestUnfocusedBorder_HasBorder(t *testing.T) {
	s := UnfocusedBorder()
	if !s.GetBorderTop() {
		t.Error("UnfocusedBorder should have a top border")
	}
}

func TestStyles_ChangeWithTheme_DarkToLight(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	// dark と light でベースカラーが異なるスタイルを検証
	styleFuncs := []struct {
		name string
		fn   func() lipgloss.Style
	}{
		{"MutedStyle", MutedStyle},
		{"StoppedStyle", StoppedStyle},
		{"ErrorStyle", ErrorStyle},
		{"ReconnectingStyle", ReconnectingStyle},
		{"DividerStyle", DividerStyle},
		{"DescStyle", DescStyle},
		{"TextStyle", TextStyle},
	}

	for _, tt := range styleFuncs {
		t.Run(tt.name, func(t *testing.T) {
			theme.Apply("dark-violet")
			s1 := tt.fn()
			theme.Apply("light-violet")
			s2 := tt.fn()
			// dark と light でベースカラーが異なるため前景色も変わるはず
			if s1.GetForeground() == s2.GetForeground() {
				t.Errorf("%s should change foreground between dark and light theme", tt.name)
			}
		})
	}
}

// --- RenderWithBorderTitle 境界値テスト ---

func TestRenderWithBorderTitle_ZeroWidth(t *testing.T) {
	style := UnfocusedBorder()
	// width=0 でもパニックしないことを確認
	rendered := RenderWithBorderTitle(style, 0, 1, "Title", "body")
	if rendered == "" {
		t.Error("should produce non-empty output even with zero width")
	}
}

func TestRenderWithBorderTitle_ZeroHeight(t *testing.T) {
	style := UnfocusedBorder()
	// height=0 でもパニックしないことを確認
	rendered := RenderWithBorderTitle(style, 20, 0, "Title", "body")
	if rendered == "" {
		t.Error("should produce non-empty output even with zero height")
	}
}

func TestRenderWithBorderTitle_EmptyContent(t *testing.T) {
	style := FocusedBorder()
	rendered := RenderWithBorderTitle(style, 20, 3, "Title", "")
	if !strings.Contains(rendered, "Title") {
		t.Error("should contain title even with empty content")
	}
}
