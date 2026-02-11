package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap はアプリケーション全体のキーバインドを定義する。
type KeyMap struct {
	// グローバルキー
	Tab    key.Binding
	Help   key.Binding
	Search key.Binding
	Escape key.Binding
	Quit   key.Binding
	ForceQuit key.Binding

	// ナビゲーション
	Up   key.Binding
	Down key.Binding

	// アクション
	Enter      key.Binding
	Disconnect key.Binding
	Delete     key.Binding
}

// DefaultKeyMap はデフォルトのキーバインドを返す。
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "ペイン切替"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "ヘルプ"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "検索"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "キャンセル"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "終了"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("Ctrl+C", "強制終了"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "上"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "下"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "実行"),
		),
		Disconnect: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "切断"),
		),
		Delete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "削除"),
		),
	}
}

// ShortHelp は help.KeyMap インターフェースを満たす。
// ヘルプバーに表示する主要キーバインドを返す。
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Help, k.Quit}
}

// FullHelp は help.KeyMap インターフェースを満たす。
// 全キーバインドをグループ分けして返す。
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.Help, k.Search, k.Escape, k.Quit, k.ForceQuit},
		{k.Up, k.Down},
		{k.Enter, k.Disconnect, k.Delete},
	}
}
