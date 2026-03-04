package molecules

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
)

// InfoDismissedMsg はインフォダイアログが閉じられたときに発行されるメッセージ。
type InfoDismissedMsg struct{}

// InfoDialog は OK ボタンのみの情報ダイアログを提供する Bubble Tea モデル。
type InfoDialog struct {
	message string
}

// NewInfoDialog は新しい InfoDialog を生成する。
func NewInfoDialog(message string) InfoDialog {
	return InfoDialog{message: message}
}

// Init は Bubble Tea の Init メソッド。
func (m InfoDialog) Init() tea.Cmd {
	return nil
}

// Update は Bubble Tea の Update メソッド。
func (m InfoDialog) Update(msg tea.Msg) (InfoDialog, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "enter", "esc", "o":
		return m, func() tea.Msg { return InfoDismissedMsg{} }
	}

	return m, nil
}

// View は Bubble Tea の View メソッド。
func (m InfoDialog) View() string {
	msg := tui.TextStyle().Render(m.message)

	okBtn := tui.SelectedStyle().Render(" " + i18n.T("tui.update.ok") + " ")

	hints := atoms.RenderKeyHint(
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", i18n.T("tui.update.ok"))),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		msg,
		"  "+okBtn,
		hints,
	)

	return tui.FocusedBorder().Render(content)
}
