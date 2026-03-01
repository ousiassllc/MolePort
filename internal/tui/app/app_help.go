package app

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui"
)

// renderHelpOverlay はヘルプモーダルを画面中央にオーバーレイ描画する。
func (m MainModel) renderHelpOverlay() string {
	lines := []string{
		tui.TitleStyle().Render("キー操作"),
		"",
		tui.KeyStyle().Render("  Tab") + tui.MutedStyle().Render("         ペイン切替 (Forwards ↔ Setup)"),
		tui.KeyStyle().Render("  /") + tui.MutedStyle().Render("           セットアップパネルにフォーカス"),
		tui.KeyStyle().Render("  ↑/k ↓/j") + tui.MutedStyle().Render("     カーソル移動"),
		tui.KeyStyle().Render("  Enter") + tui.MutedStyle().Render("       選択 / 接続トグル"),
		tui.KeyStyle().Render("  d") + tui.MutedStyle().Render("           切断"),
		tui.KeyStyle().Render("  x") + tui.MutedStyle().Render("           ルール削除"),
		tui.KeyStyle().Render("  Esc") + tui.MutedStyle().Render("         ウィザードキャンセル"),
		tui.KeyStyle().Render("  t") + tui.MutedStyle().Render("           テーマ選択"),
		tui.KeyStyle().Render("  l") + tui.MutedStyle().Render("           言語切替"),
		tui.KeyStyle().Render("  v") + tui.MutedStyle().Render("           バージョン表示"),
		tui.KeyStyle().Render("  ?") + tui.MutedStyle().Render("           ヘルプ"),
		tui.KeyStyle().Render("  q / Ctrl+C") + tui.MutedStyle().Render("  終了"),
		"",
		tui.MutedStyle().Render("  任意のキーで閉じる"),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	dialog := tui.FocusedBorder().Render(content)
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}
