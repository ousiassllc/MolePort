package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

// カラーパレット（テーマから動的取得）

// AccentColor はアクセントカラーを返す。
func AccentColor() lipgloss.Color { return theme.Current().Accent }

// TextColor はテキストカラーを返す。
func TextColor() lipgloss.Color { return theme.Current().Text }

// MutedColor はミュートカラーを返す。
func MutedColor() lipgloss.Color { return theme.Current().Muted }

// ErrorColor はエラーカラーを返す。
func ErrorColor() lipgloss.Color { return theme.Current().Error }

// WarningColor は警告カラーを返す。
func WarningColor() lipgloss.Color { return theme.Current().Warning }

// テキストスタイル

// TitleStyle はタイトル用の太字アクセントスタイルを返す。
func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Current().Accent)
}

// MutedStyle はミュート（低強調）テキスト用スタイルを返す。
func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Muted)
}

// SelectedStyle は選択中アイテムのハイライトスタイルを返す。
func SelectedStyle() lipgloss.Style {
	p := theme.Current()
	return lipgloss.NewStyle().Background(p.BgHighlight).Foreground(p.Accent).Bold(true)
}

// TextStyle は標準テキスト用スタイルを返す。
func TextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Text)
}

// ステータスカラースタイル

// ActiveStyle はアクティブ状態のスタイルを返す。
func ActiveStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Accent) }

// StoppedStyle は停止状態のスタイルを返す。
func StoppedStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Muted) }

// ErrorStyle はエラー状態のスタイルを返す。
func ErrorStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Error) }

// ReconnectingStyle は再接続中のスタイルを返す。
func ReconnectingStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Warning)
}

// WarningStyle は警告メッセージのスタイルを返す。
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Warning).Bold(true)
}

// キーヒントスタイル

// KeyStyle はキーバインド表示用のスタイルを返す。
func KeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Accent).Bold(true)
}

// DescStyle はキーバインド説明文のスタイルを返す。
func DescStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Muted) }

// 区切り線スタイル

// DividerStyle は区切り線のスタイルを返す。
func DividerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Dim) }

// ヘッダースタイル

// HeaderStyle はヘッダー用の太字アクセントスタイルを返す。
func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Current().Accent)
}

// パネルボーダースタイル

// FocusedBorder はフォーカス中のパネルボーダースタイルを返す。
func FocusedBorder() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Current().Accent).
		Padding(0, 1)
}

// UnfocusedBorder は非フォーカスのパネルボーダースタイルを返す。
func UnfocusedBorder() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Current().Dim).
		Padding(0, 1)
}

// StatusBarStyle はステータスバーのスタイルを返す。
func StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 1)
}

// RenderWithBorderTitle はボーダー付きでコンテンツを描画し、上辺にインラインタイトルを埋め込む。
func RenderWithBorderTitle(style lipgloss.Style, width, height int, title, content string) string {
	rendered := style.Width(width).Height(height).Render(content)
	if title == "" {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}

	// 上辺ボーダー行をタイトル入りで再構築する
	topWidth := lipgloss.Width(lines[0])
	borderFg := style.GetBorderTopForeground()
	borderColor := lipgloss.NewStyle().Foreground(borderFg)

	// NOTE: RoundedBorder 前提。他のボーダースタイルでは正しく動作しない。
	b := lipgloss.RoundedBorder()
	prefix := borderColor.Render(b.TopLeft+b.Top) + " " + title + " "
	prefixWidth := lipgloss.Width(prefix)

	suffix := borderColor.Render(b.TopRight)
	suffixWidth := lipgloss.Width(suffix)
	fillCount := max(topWidth-prefixWidth-suffixWidth, 0)

	lines[0] = prefix + borderColor.Render(strings.Repeat(b.Top, fillCount)+b.TopRight)
	return strings.Join(lines, "\n")
}
