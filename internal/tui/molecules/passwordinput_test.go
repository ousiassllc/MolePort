package molecules

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPasswordInput_InitialState(t *testing.T) {
	pi := NewPasswordInput()
	if pi.Active() {
		t.Error("new PasswordInput should not be active")
	}
	if v := pi.View(); v != "" {
		t.Errorf("inactive PasswordInput should render empty, got %q", v)
	}
}

func TestPasswordInput_ShowAndHide(t *testing.T) {
	pi := NewPasswordInput()

	pi.Show("Password:")
	if !pi.Active() {
		t.Error("PasswordInput should be active after Show")
	}
	if v := pi.View(); v == "" {
		t.Error("active PasswordInput should render non-empty")
	}

	pi.Hide()
	if pi.Active() {
		t.Error("PasswordInput should not be active after Hide")
	}
}

func TestPasswordInput_EnterSubmitsValue(t *testing.T) {
	pi := NewPasswordInput()
	pi.Show("Password:")

	// テキスト入力をシミュレート
	var cmd tea.Cmd
	for _, r := range "secret" {
		pi, cmd = pi.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}

	// Enter で送信
	pi, cmd = pi.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}

	msg := cmd()
	submit, ok := msg.(PasswordSubmitMsg)
	if !ok {
		t.Fatalf("expected PasswordSubmitMsg, got %T", msg)
	}
	if submit.Value != "secret" {
		t.Errorf("value = %q, want %q", submit.Value, "secret")
	}
	if submit.Cancelled {
		t.Error("submit should not be cancelled")
	}
}

func TestPasswordInput_EscCancels(t *testing.T) {
	pi := NewPasswordInput()
	pi.Show("Password:")

	pi, cmd := pi.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}

	msg := cmd()
	submit, ok := msg.(PasswordSubmitMsg)
	if !ok {
		t.Fatalf("expected PasswordSubmitMsg, got %T", msg)
	}
	if !submit.Cancelled {
		t.Error("Esc should produce a cancelled submit")
	}
}

func TestPasswordInput_InactiveIgnoresInput(t *testing.T) {
	pi := NewPasswordInput()

	pi, cmd := pi.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("inactive PasswordInput should not produce commands")
	}
	if pi.Active() {
		t.Error("should remain inactive")
	}
}
