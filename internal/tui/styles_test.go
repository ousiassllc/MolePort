package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderWithBorderTitle_EmptyTitle(t *testing.T) {
	style := UnfocusedBorder
	rendered := RenderWithBorderTitle(style, 20, 3, "", "hello")
	plain := style.Width(20).Height(3).Render("hello")
	if rendered != plain {
		t.Errorf("empty title should return same output as plain render")
	}
}

func TestRenderWithBorderTitle_HasTitle(t *testing.T) {
	style := FocusedBorder
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
	style := UnfocusedBorder
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
	style := UnfocusedBorder
	longTitle := strings.Repeat("A", 100)
	// Should not panic even if title exceeds border width
	rendered := RenderWithBorderTitle(style, 20, 3, longTitle, "body")
	if rendered == "" {
		t.Error("should produce non-empty output even with long title")
	}
}
