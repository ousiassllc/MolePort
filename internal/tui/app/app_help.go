package app

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
)

// renderHelpOverlay はヘルプモーダルを画面中央にオーバーレイ描画する。
func (m MainModel) renderHelpOverlay() string {
	lines := []string{
		tui.TitleStyle().Render(i18n.T("tui.help.title")),
		"",
		tui.KeyStyle().Render("  Tab") + tui.MutedStyle().Render("         "+i18n.T("tui.help.tab")),
		tui.KeyStyle().Render("  /") + tui.MutedStyle().Render("           "+i18n.T("tui.help.slash")),
		tui.KeyStyle().Render("  ↑/k ↓/j") + tui.MutedStyle().Render("     "+i18n.T("tui.help.arrows")),
		tui.KeyStyle().Render("  Enter") + tui.MutedStyle().Render("       "+i18n.T("tui.help.enter")),
		tui.KeyStyle().Render("  d") + tui.MutedStyle().Render("           "+i18n.T("tui.help.d")),
		tui.KeyStyle().Render("  x") + tui.MutedStyle().Render("           "+i18n.T("tui.help.x")),
		tui.KeyStyle().Render("  Esc") + tui.MutedStyle().Render("         "+i18n.T("tui.help.esc")),
		tui.KeyStyle().Render("  t") + tui.MutedStyle().Render("           "+i18n.T("tui.help.t")),
		tui.KeyStyle().Render("  l") + tui.MutedStyle().Render("           "+i18n.T("tui.help.l")),
		tui.KeyStyle().Render("  v") + tui.MutedStyle().Render("           "+i18n.T("tui.help.v")),
		tui.KeyStyle().Render("  ?") + tui.MutedStyle().Render("           "+i18n.T("tui.help.question")),
		tui.KeyStyle().Render("  q / Ctrl+C") + tui.MutedStyle().Render("  "+i18n.T("tui.help.q")),
		"",
		tui.MutedStyle().Render("  " + i18n.T("tui.help.any_key_close")),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	dialog := tui.FocusedBorder().Render(content)
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}
