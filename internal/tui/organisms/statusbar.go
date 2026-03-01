package organisms

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/i18n"
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
	warning     string
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

// SetWarning は警告テキストを設定する。空文字列で警告を解除する。
func (s *StatusBar) SetWarning(text string) {
	s.warning = text
}

// SetWidth は表示幅を設定する。
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// View はステータスバーを描画する。
func (s StatusBar) View() string {
	sep := tui.DividerStyle().Render(" │ ")

	stats := fmt.Sprintf(
		"%s %s  %s %s%s%s %s  %s %s",
		tui.ActiveStyle().Render(fmt.Sprintf("%d", s.stats.TotalHosts)),
		i18n.T("tui.statusbar.hosts"),
		tui.ActiveStyle().Render(fmt.Sprintf("%d", s.stats.ConnectedHosts)),
		i18n.T("tui.statusbar.connected"),
		sep,
		tui.ActiveStyle().Render(fmt.Sprintf("%d", s.stats.TotalForwards)),
		i18n.T("tui.statusbar.forwards"),
		tui.ActiveStyle().Render(fmt.Sprintf("%d", s.stats.ActiveForwards)),
		i18n.T("tui.statusbar.active"),
	)

	// ペインに応じたキーヒント
	var contextHints string
	switch s.focusedPane {
	case tui.PaneForwards:
		contextHints = fmt.Sprintf(
			"%s %s  %s %s  %s %s",
			tui.KeyStyle().Render("[Enter]"), tui.DescStyle().Render(i18n.T("tui.keys.toggle")),
			tui.KeyStyle().Render("[d]"), tui.DescStyle().Render(i18n.T("tui.keys.disconnect")),
			tui.KeyStyle().Render("[x]"), tui.DescStyle().Render(i18n.T("tui.keys.delete")),
		)
	case tui.PaneSetup:
		contextHints = fmt.Sprintf(
			"%s %s  %s %s",
			tui.KeyStyle().Render("[Enter]"), tui.DescStyle().Render(i18n.T("tui.keys.select")),
			tui.KeyStyle().Render("[Esc]"), tui.DescStyle().Render(i18n.T("tui.keys.cancel")),
		)
	}

	globalHints := fmt.Sprintf(
		"%s %s  %s %s  %s %s",
		tui.KeyStyle().Render("[Tab]"), tui.DescStyle().Render(i18n.T("tui.keys.switch_pane")),
		tui.KeyStyle().Render("[?]"), tui.DescStyle().Render(i18n.T("tui.keys.help")),
		tui.KeyStyle().Render("[q]"), tui.DescStyle().Render(i18n.T("tui.keys.quit")),
	)

	hints := globalHints
	if contextHints != "" {
		hints = contextHints + sep + globalHints
	}

	var warningText string
	if s.warning != "" {
		warningText = sep + tui.WarningStyle().Render(s.warning)
	}

	left := tui.MutedStyle().Render(" ") + stats + warningText
	right := hints

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
