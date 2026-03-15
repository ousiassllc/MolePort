package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// KeyMap はアプリケーション全体のキーバインドを定義する。
type KeyMap struct {
	// グローバルキー
	Tab       key.Binding
	Help      key.Binding
	Search    key.Binding
	Escape    key.Binding
	Quit      key.Binding
	ForceQuit key.Binding

	// ナビゲーション
	Up   key.Binding
	Down key.Binding

	// アクション
	Enter      key.Binding
	Disconnect key.Binding
	Delete     key.Binding
	Theme      key.Binding
	Lang       key.Binding
	Version    key.Binding
}

// DefaultKeyMap はデフォルトのキーバインドを返す。
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", i18n.T("tui.keys.switch_pane")),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", i18n.T("tui.keys.help")),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", i18n.T("tui.keys.search")),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", i18n.T("tui.keys.cancel")),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", i18n.T("tui.keys.quit")),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("Ctrl+C", i18n.T("tui.keys.force_quit")),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", i18n.T("tui.keys.up")),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", i18n.T("tui.keys.down")),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", i18n.T("tui.keys.execute")),
		),
		Disconnect: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", i18n.T("tui.keys.disconnect")),
		),
		Delete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", i18n.T("tui.keys.delete")),
		),
		Theme: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", i18n.T("tui.keys.theme")),
		),
		Lang: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", i18n.T("tui.keys.lang")),
		),
		Version: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", i18n.T("tui.keys.version")),
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
		{k.Enter, k.Disconnect, k.Delete, k.Theme, k.Lang, k.Version},
	}
}
