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
	// ステータスバー内の各スタイルは Background(BgHighlight) が必要。
	// 内部のANSIリセットが外側の背景色を打ち消すため、各テキスト片に個別適用する。
	bg := tui.BgHighlight
	accentBg := tui.ActiveStyle.Background(bg)
	keyBg := tui.KeyStyle.Background(bg)
	descBg := tui.DescStyle.Background(bg)
	mutedBg := tui.MutedStyle.Background(bg)
	dimBg := tui.DividerStyle.Background(bg)
	textBg := lipgloss.NewStyle().Background(bg)

	sep := dimBg.Render(" │ ")

	stats := fmt.Sprintf(
		"%s%s%s%s%s%s%s%s%s",
		accentBg.Render(fmt.Sprintf("%d", s.stats.TotalHosts)),
		mutedBg.Render(" hosts  "),
		accentBg.Render(fmt.Sprintf("%d", s.stats.ConnectedHosts)),
		mutedBg.Render(" connected"),
		sep,
		accentBg.Render(fmt.Sprintf("%d", s.stats.TotalForwards)),
		mutedBg.Render(" forwards  "),
		accentBg.Render(fmt.Sprintf("%d", s.stats.ActiveForwards)),
		mutedBg.Render(" active"),
	)

	// ペインに応じたキーヒント
	var contextHints string
	switch s.focusedPane {
	case tui.PaneForwards:
		contextHints = fmt.Sprintf(
			"%s %s  %s %s  %s %s",
			keyBg.Render("[Enter]"), descBg.Render("Toggle"),
			keyBg.Render("[d]"), descBg.Render("Disconnect"),
			keyBg.Render("[x]"), descBg.Render("Delete"),
		)
	case tui.PaneSetup:
		contextHints = fmt.Sprintf(
			"%s %s  %s %s",
			keyBg.Render("[Enter]"), descBg.Render("Select"),
			keyBg.Render("[Esc]"), descBg.Render("Cancel"),
		)
	}

	globalHints := fmt.Sprintf(
		"%s %s  %s %s  %s %s",
		keyBg.Render("[Tab]"), descBg.Render("Switch"),
		keyBg.Render("[?]"), descBg.Render("Help"),
		keyBg.Render("[q]"), descBg.Render("Quit"),
	)

	hints := globalHints
	if contextHints != "" {
		hints = contextHints + sep + globalHints
	}

	left := stats
	right := hints

	// StatusBarStyle の Padding(0,1) 分を差し引いたコンテンツ幅
	contentWidth := s.width - 2
	if contentWidth <= 0 {
		return tui.StatusBarStyle.Render(left + sep + right)
	}

	gap := contentWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 3 {
		return tui.StatusBarStyle.Width(contentWidth).Render(left)
	}

	padding := textBg.Width(gap).Render("")
	return tui.StatusBarStyle.Width(contentWidth).Render(left + padding + right)
}
