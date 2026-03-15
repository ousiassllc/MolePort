package molecules

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptInput_EnterSubmitsValue(t *testing.T) {
	pi := NewPromptInput()
	pi.Focus() // textinput はフォーカスしないとキー入力を受け付けない

	// テキスト入力をシミュレート
	for _, r := range "hello" {
		pi, _ = pi.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if pi.Value() != "hello" {
		t.Fatalf("value = %q, want %q", pi.Value(), "hello")
	}

	// Enter で送信
	pi, cmd := pi.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}

	msg := cmd()
	submit, ok := msg.(PromptSubmitMsg)
	if !ok {
		t.Fatalf("expected PromptSubmitMsg, got %T", msg)
	}
	if submit.Value != "hello" {
		t.Errorf("submitted value = %q, want %q", submit.Value, "hello")
	}

	// 送信後にフィールドがリセットされていることを確認
	if pi.Value() != "" {
		t.Errorf("value after submit = %q, want empty", pi.Value())
	}
}

func TestPromptInput_EmptyEnterIgnored(t *testing.T) {
	pi := NewPromptInput()
	pi.Focus()

	// 空の状態で Enter → コマンドなし（PromptSubmitMsg は生成されない）
	_, cmd := pi.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(PromptSubmitMsg); ok {
			t.Error("empty Enter should not produce PromptSubmitMsg")
		}
	}
}

func TestPromptInput_FocusedState(t *testing.T) {
	pi := NewPromptInput()
	if pi.Focused() {
		t.Error("new PromptInput should not be focused")
	}

	pi.Focus()
	if !pi.Focused() {
		t.Error("PromptInput should be focused after Focus()")
	}

	pi.Blur()
	if pi.Focused() {
		t.Error("PromptInput should not be focused after Blur()")
	}
}
