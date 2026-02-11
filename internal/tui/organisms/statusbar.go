package organisms

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui"
)

// StatusBarStats はステータスバーに表示する統計情報。
type StatusBarStats struct {
	TotalHosts     int
	ConnectedHosts int
	TotalForwards  int
	ActiveForwards int
}

// StatusBar はアプリケーション下部に表示するステータスバー。
type StatusBar struct {
	stats       StatusBarStats
	focusedPane tui.FocusPane
	width       int
}

// NewStatusBar は新しい StatusBar を生成する。
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// SetStats は統計情報を更新する。
func (s *StatusBar) SetStats(stats StatusBarStats) {
	s.stats = stats
}

// SetFocusedPane はフォーカス中のペインを更新する。
func (s *StatusBar) SetFocusedPane(pane tui.FocusPane) {
	s.focusedPane = pane
}

// SetWidth は表示幅を設定する。
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// View はステータスバーを描画する。
func (s StatusBar) View() string {
	sep := tui.DividerStyle.Render(" │ ")

	stats := fmt.Sprintf(
		"%s hosts  %s connected%s%s forwards  %s active",
		tui.ActiveStyle.Render(fmt.Sprintf("%d", s.stats.TotalHosts)),
		tui.ActiveStyle.Render(fmt.Sprintf("%d", s.stats.ConnectedHosts)),
		sep,
		tui.ActiveStyle.Render(fmt.Sprintf("%d", s.stats.TotalForwards)),
		tui.ActiveStyle.Render(fmt.Sprintf("%d", s.stats.ActiveForwards)),
	)

	// ペインに応じたキーヒント
	var contextHints string
	switch s.focusedPane {
	case tui.PaneForwards:
		contextHints = fmt.Sprintf(
			"%s %s  %s %s  %s %s",
			tui.KeyStyle.Render("[Enter]"), tui.DescStyle.Render("Toggle"),
			tui.KeyStyle.Render("[d]"), tui.DescStyle.Render("Disconnect"),
			tui.KeyStyle.Render("[x]"), tui.DescStyle.Render("Delete"),
		)
	case tui.PaneSetup:
		contextHints = fmt.Sprintf(
			"%s %s  %s %s",
			tui.KeyStyle.Render("[Enter]"), tui.DescStyle.Render("Select"),
			tui.KeyStyle.Render("[Esc]"), tui.DescStyle.Render("Cancel"),
		)
	}

	globalHints := fmt.Sprintf(
		"%s %s  %s %s  %s %s",
		tui.KeyStyle.Render("[Tab]"), tui.DescStyle.Render("Switch"),
		tui.KeyStyle.Render("[?]"), tui.DescStyle.Render("Help"),
		tui.KeyStyle.Render("[q]"), tui.DescStyle.Render("Quit"),
	)

	hints := globalHints
	if contextHints != "" {
		hints = contextHints + sep + globalHints
	}

	left := tui.MutedStyle.Render(" ") + stats
	right := hints

	// 幅が足りない場合は統計のみ表示
	if s.width <= 0 {
		return left + sep + right
	}

	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 3 {
		return left
	}

	padding := lipgloss.NewStyle().Width(gap).Render("")
	return left + padding + right
}
