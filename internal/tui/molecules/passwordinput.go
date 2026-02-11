package molecules

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui"
)

// PasswordSubmitMsg はパスワード入力が確定されたときに発行される。
type PasswordSubmitMsg struct {
	Value     string
	Cancelled bool
}

// PasswordInput はパスワード入力欄を提供する Bubble Tea モデル。
// 入力文字はマスクされる。
type PasswordInput struct {
	textInput textinput.Model
	prompt    string
	active    bool
}

// NewPasswordInput は新しい PasswordInput を生成する。
func NewPasswordInput() PasswordInput {
	ti := textinput.New()
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.CharLimit = 256
	return PasswordInput{textInput: ti}
}

// Show はパスワード入力を表示し、フォーカスする。
func (m *PasswordInput) Show(prompt string) tea.Cmd {
	m.prompt = prompt
	m.active = true
	m.textInput.Reset()
	m.textInput.Prompt = tui.ActiveStyle.Render("> ") + " "
	return m.textInput.Focus()
}

// Hide はパスワード入力を非表示にする。
func (m *PasswordInput) Hide() {
	m.active = false
	m.textInput.Blur()
	m.textInput.Reset()
}

// Active はパスワード入力が表示中かどうかを返す。
func (m PasswordInput) Active() bool {
	return m.active
}

// Update は Bubble Tea の Update メソッド。
func (m PasswordInput) Update(msg tea.Msg) (PasswordInput, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEnter:
			value := m.textInput.Value()
			m.Hide()
			return m, func() tea.Msg {
				return PasswordSubmitMsg{Value: value}
			}
		case tea.KeyEsc, tea.KeyCtrlC:
			m.Hide()
			return m, func() tea.Msg {
				return PasswordSubmitMsg{Cancelled: true}
			}
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View は PasswordInput を描画する。
func (m PasswordInput) View() string {
	if !m.active {
		return ""
	}

	prompt := tui.TextStyle.Render(m.prompt)
	input := m.textInput.View()
	hints := tui.MutedStyle.Render("[Enter] 送信  [Esc] キャンセル")

	content := lipgloss.JoinVertical(lipgloss.Left,
		prompt,
		input,
		hints,
	)

	return tui.DialogStyle.Render(content)
}
