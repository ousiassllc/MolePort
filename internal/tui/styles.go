package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

// カラーパレット（テーマから動的取得）

func AccentColor() lipgloss.Color  { return theme.Current().Accent }
func TextColor() lipgloss.Color    { return theme.Current().Text }
func MutedColor() lipgloss.Color   { return theme.Current().Muted }
func ErrorColor() lipgloss.Color   { return theme.Current().Error }
func WarningColor() lipgloss.Color { return theme.Current().Warning }

// テキストスタイル

func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Current().Accent)
}

func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Muted)
}

func SelectedStyle() lipgloss.Style {
	p := theme.Current()
	return lipgloss.NewStyle().Background(p.BgHighlight).Foreground(p.Accent).Bold(true)
}

func TextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Text)
}

// ステータスカラースタイル

func ActiveStyle() lipgloss.Style  { return lipgloss.NewStyle().Foreground(theme.Current().Accent) }
func StoppedStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Muted) }
func ErrorStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(theme.Current().Error) }
func ReconnectingStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Warning)
}
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Warning).Bold(true)
}

// キーヒントスタイル

func KeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(theme.Current().Accent).Bold(true)
}
func DescStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Muted) }

// 区切り線スタイル

func DividerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Current().Dim) }

// ヘッダースタイル

func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Current().Accent)
}

// パネルボーダースタイル

func FocusedBorder() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Current().Accent).
		Padding(0, 1)
}

func UnfocusedBorder() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Current().Dim).
		Padding(0, 1)
}

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
	fillCount := topWidth - prefixWidth - suffixWidth
	if fillCount < 0 {
		fillCount = 0
	}

	lines[0] = prefix + borderColor.Render(strings.Repeat(b.Top, fillCount)+b.TopRight)
	return strings.Join(lines, "\n")
}
