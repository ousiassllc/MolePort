package organisms

import (
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
