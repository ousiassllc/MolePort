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
	stats := fmt.Sprintf(
		"%d hosts  %d connected │ %d forwards  %d active",
		s.stats.TotalHosts,
		s.stats.ConnectedHosts,
		s.stats.TotalForwards,
		s.stats.ActiveForwards,
	)

	hints := "[Tab] Switch  [/] Command  [?] Help  [q] Quit"

	left := tui.MutedStyle.Render(stats)
	right := tui.MutedStyle.Render(hints)

	// 幅が足りない場合は統計のみ表示
	if s.width <= 0 {
		return left + " │ " + right
	}

	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 3 {
		return left
	}

	padding := lipgloss.NewStyle().Width(gap).Render("")
	return left + padding + right
}
