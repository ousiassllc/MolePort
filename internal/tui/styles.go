package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// カラーパレット（単一アクセントカラー: バイオレット + グレースケール）
var (
	Accent      = lipgloss.Color("#7C3AED") // バイオレット: フォーカス、選択、アクティブ
	AccentDim   = lipgloss.Color("#6D28D9") // やや暗いアクセント: セカンダリ
	Text        = lipgloss.Color("#E4E4E7") // 通常テキスト（薄灰）
	Muted       = lipgloss.Color("#71717A") // 補助テキスト、ラベル
	Dim         = lipgloss.Color("#3F3F46") // ボーダー、区切り線
	Error       = lipgloss.Color("#EF4444") // エラー（唯一の例外色）
	Warning     = lipgloss.Color("#F59E0B") // 再接続中
	BgHighlight = lipgloss.Color("#27272A") // 選択行の背景
)

// テキストスタイル
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Accent)

	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	SelectedStyle = lipgloss.NewStyle().
			Background(BgHighlight).
			Foreground(Accent).
			Bold(true)

	TextStyle = lipgloss.NewStyle().
			Foreground(Text)
)

// ステータスカラースタイル
var (
	ActiveStyle       = lipgloss.NewStyle().Foreground(Accent)
	StoppedStyle      = lipgloss.NewStyle().Foreground(Muted)
	ErrorStyle        = lipgloss.NewStyle().Foreground(Error)
	ReconnectingStyle = lipgloss.NewStyle().Foreground(Warning)
)

// キーヒントスタイル
var (
	KeyStyle  = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	DescStyle = lipgloss.NewStyle().Foreground(Muted)
)

// 区切り線スタイル
var DividerStyle = lipgloss.NewStyle().Foreground(Dim)

// ヘッダースタイル
var HeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(Accent)

// セクションタイトルスタイル
var SectionTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(Accent)

// フォーカスインジケーター
var FocusIndicator = lipgloss.NewStyle().
	Foreground(Accent).
	Bold(true).
	Render("▌")

// ダイアログスタイル（ボーダーなし、パディングのみ）
var DialogStyle = lipgloss.NewStyle().
	Padding(0, 1)

// パネルボーダースタイル
var (
	FocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Accent).
			Padding(0, 1)

	UnfocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Dim).
			Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Background(BgHighlight).
			Padding(0, 1)
)

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
