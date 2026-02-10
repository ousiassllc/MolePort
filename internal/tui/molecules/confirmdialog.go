package molecules

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
)

// ConfirmResultMsg は確認ダイアログの結果を通知するメッセージ。
type ConfirmResultMsg struct {
	Confirmed bool
}

// ConfirmDialog は Yes/No の確認ダイアログを提供する Bubble Tea モデル。
type ConfirmDialog struct {
	message string
	focused bool // true = Yes にフォーカス
}

// NewConfirmDialog は新しい ConfirmDialog を生成する。
func NewConfirmDialog(message string) ConfirmDialog {
	return ConfirmDialog{
		message: message,
		focused: false, // デフォルトは No（安全側）
	}
}

// Init は Bubble Tea の Init メソッド。
func (m ConfirmDialog) Init() tea.Cmd {
	return nil
}

// Update は Bubble Tea の Update メソッド。
func (m ConfirmDialog) Update(msg tea.Msg) (ConfirmDialog, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "left", "h", "right", "l", "tab":
		m.focused = !m.focused
	case "y":
		return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: true} }
	case "n", "esc":
		return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: false} }
	case "enter":
		return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: m.focused} }
	}

	return m, nil
}

// View は Bubble Tea の View メソッド。
func (m ConfirmDialog) View() string {
	msg := tui.TitleStyle.Render(m.message)

	yesStyle := tui.MutedStyle
	noStyle := tui.MutedStyle
	if m.focused {
		yesStyle = tui.SelectedStyle
	} else {
		noStyle = tui.SelectedStyle
	}

	buttons := fmt.Sprintf("  %s  %s",
		yesStyle.Render(" Yes "),
		noStyle.Render(" No "),
	)

	hints := atoms.RenderKeyHint(
		key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "はい")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "いいえ")),
		key.NewBinding(key.WithKeys("←/→"), key.WithHelp("←/→", "切替")),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		msg,
		buttons,
		"",
		hints,
	)

	return tui.PanelBorder.Render(content)
}
