package organisms

import (
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/tui"
)

func TestStatusBar_SetStats(t *testing.T) {
	sb := NewStatusBar()
	sb.SetStats(StatusBarStats{
		TotalHosts:     3,
		ConnectedHosts: 2,
		TotalForwards:  5,
		ActiveForwards: 4,
	})
	sb.SetWidth(120)

	view := sb.View()
	if view == "" {
		t.Error("StatusBar.View() should not be empty")
	}
}

func TestStatusBar_SetWarning(t *testing.T) {
	sb := NewStatusBar()
	sb.SetWidth(120)
	sb.SetWarning("test warning")

	view := sb.View()
	if view == "" {
		t.Error("StatusBar.View() with warning should not be empty")
	}
}

func TestStatusBar_SetFocusedPane(t *testing.T) {
	sb := NewStatusBar()
	sb.SetWidth(120)

	// フォーカスペインの切替でパニックしないことを確認
	for _, pane := range []tui.FocusPane{tui.PaneForwards, tui.PaneSetup} {
		sb.SetFocusedPane(pane)
		view := sb.View()
		if view == "" {
			t.Errorf("StatusBar.View() with pane %d should not be empty", pane)
		}
	}
}

func TestStatusBar_NarrowWidth(t *testing.T) {
	sb := NewStatusBar()
	sb.SetStats(StatusBarStats{TotalHosts: 1, ConnectedHosts: 1})
	sb.SetWidth(10) // 非常に狭い幅

	view := sb.View()
	if view == "" {
		t.Error("StatusBar.View() with narrow width should not be empty")
	}
}

func TestNewStatusBar_And_SetWidth(t *testing.T) {
	sb := NewStatusBar()
	if sb.width != 0 || sb.warning != "" {
		t.Error("NewStatusBar should have zero width and empty warning")
	}
	sb.SetWidth(200)
	if sb.width != 200 {
		t.Errorf("SetWidth: got %d, want 200", sb.width)
	}
	// width=0 should also produce output
	sb2 := NewStatusBar()
	sb2.SetStats(StatusBarStats{TotalHosts: 2})
	if sb2.View() == "" {
		t.Error("View with zero width should not be empty")
	}
}

func TestStatusBar_View_ContainsCounts(t *testing.T) {
	sb := NewStatusBar()
	sb.SetStats(StatusBarStats{TotalHosts: 5, ConnectedHosts: 3, TotalForwards: 7, ActiveForwards: 2})
	sb.SetWidth(200)
	view := sb.View()
	for _, want := range []string{"5", "3", "7", "2"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() should contain %q", want)
		}
	}
}

func TestStatusBar_View_Warning(t *testing.T) {
	sb := NewStatusBar()
	sb.SetWidth(200)
	sb.SetWarning("disk full")
	if !strings.Contains(sb.View(), "disk full") {
		t.Error("View() should contain warning text")
	}
	sb.SetWarning("")
	if strings.Contains(sb.View(), "disk full") {
		t.Error("View() should not contain cleared warning")
	}
}
