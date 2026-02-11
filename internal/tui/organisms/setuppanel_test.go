package organisms

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
)

func TestSetupPanel_TextInputAcceptsKeystrokes(t *testing.T) {
	// セットアップ: ホストを1つ用意してウィザードを StepLocalPort まで進める
	p := NewSetupPanel()
	p.focused = true
	p.hosts = []core.SSHHost{
		{Name: "test-host", User: "user", HostName: "example.com", Port: 22},
	}

	// StepIdle → Enter でホスト選択 → StepSelectType
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	p, _ = p.Update(enterMsg)
	if p.step != StepSelectType {
		t.Fatalf("expected StepSelectType, got %d", p.step)
	}

	// StepSelectType → Enter で Local 選択 → StepLocalPort
	p, _ = p.Update(enterMsg)
	if p.step != StepLocalPort {
		t.Fatalf("expected StepLocalPort, got %d", p.step)
	}

	// 数字キーを入力
	for _, r := range "8080" {
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		p, _ = p.Update(keyMsg)
	}

	got := p.portInput.Value()
	if got != "8080" {
		t.Errorf("expected portInput value %q, got %q", "8080", got)
	}
}

func TestSetupPanel_TextInputHostAcceptsKeystrokes(t *testing.T) {
	p := NewSetupPanel()
	p.focused = true
	p.hosts = []core.SSHHost{
		{Name: "test-host", User: "user", HostName: "example.com", Port: 22},
	}

	// StepIdle → StepSelectType → StepLocalPort → StepRemoteHost
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	p, _ = p.Update(enterMsg) // → StepSelectType
	p, _ = p.Update(enterMsg) // → StepLocalPort

	// ポート番号入力
	for _, r := range "3000" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	p, _ = p.Update(enterMsg) // → StepRemoteHost

	if p.step != StepRemoteHost {
		t.Fatalf("expected StepRemoteHost, got %d", p.step)
	}

	// ホスト名入力
	for _, r := range "db.local" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	got := p.hostInput.Value()
	if got != "db.local" {
		t.Errorf("expected hostInput value %q, got %q", "db.local", got)
	}
}
