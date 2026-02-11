package molecules

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
)

// PromptSubmitMsg はプロンプトで入力が確定されたときに発行されるメッセージ。
type PromptSubmitMsg struct {
	Value string
}

// PromptInput はコマンド入力欄を提供する Bubble Tea モデル。
type PromptInput struct {
	textInput textinput.Model
}

// NewPromptInput は新しい PromptInput を生成する。
func NewPromptInput() PromptInput {
	ti := textinput.New()
	ti.Prompt = tui.ActiveStyle.Render("> ") + " "
	ti.Placeholder = "コマンドを入力..."
	ti.CharLimit = 256
	return PromptInput{textInput: ti}
}

// Init は Bubble Tea の Init メソッド。
func (m PromptInput) Init() tea.Cmd {
	return textinput.Blink
}

// Update は Bubble Tea の Update メソッド。
func (m PromptInput) Update(msg tea.Msg) (PromptInput, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.Type == tea.KeyEnter && m.textInput.Value() != "" {
			value := m.textInput.Value()
			m.textInput.Reset()
			return m, func() tea.Msg { return PromptSubmitMsg{Value: value} }
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View は Bubble Tea の View メソッド。
func (m PromptInput) View() string {
	hints := atoms.RenderKeyHint(
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "実行")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "キャンセル")),
	)
	return m.textInput.View() + "  " + hints
}

// Focus はテキスト入力にフォーカスを設定する。
func (m *PromptInput) Focus() tea.Cmd {
	return m.textInput.Focus()
}

// Blur はテキスト入力のフォーカスを解除する。
func (m *PromptInput) Blur() {
	m.textInput.Blur()
}

// Focused はフォーカス状態を返す。
func (m PromptInput) Focused() bool {
	return m.textInput.Focused()
}

// Value は現在の入力値を返す。
func (m PromptInput) Value() string {
	return m.textInput.Value()
}
