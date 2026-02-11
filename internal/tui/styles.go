package tui

import "github.com/charmbracelet/lipgloss"

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
