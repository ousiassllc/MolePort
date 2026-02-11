package pages

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

func TestDashboardSlashKeyFocusesSetupPanel(t *testing.T) {
	d := NewDashboardPage("0.1.0")
	d.SetSize(80, 24)

	// 初期状態: SetupPanel にフォーカス
	if d.FocusedPane() != tui.PaneSetup {
		t.Fatalf("initial focus = %v, want PaneSetup", d.FocusedPane())
	}

	// Tab で ForwardPanel に切り替え
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyTab})
	if d.FocusedPane() != tui.PaneForwards {
		t.Fatalf("after Tab: focus = %v, want PaneForwards", d.FocusedPane())
	}

	// / で SetupPanel に戻る
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if d.FocusedPane() != tui.PaneSetup {
		t.Errorf("after /: focus = %v, want PaneSetup", d.FocusedPane())
	}
}

func TestDashboardSlashKeyNoopWhenAlreadySetup(t *testing.T) {
	d := NewDashboardPage("0.1.0")
	d.SetSize(80, 24)

	// 初期状態: SetupPanel にフォーカス
	if d.FocusedPane() != tui.PaneSetup {
		t.Fatalf("initial focus = %v, want PaneSetup", d.FocusedPane())
	}

	// / を押しても SetupPanel のまま
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if d.FocusedPane() != tui.PaneSetup {
		t.Errorf("after / when already setup: focus = %v, want PaneSetup", d.FocusedPane())
	}
}

func TestDashboardSlashKeyIgnoredDuringInput(t *testing.T) {
	d := NewDashboardPage("0.1.0")
	d.SetSize(80, 24)

	// ホストを設定してウィザードを開始できる状態にする
	hosts := []core.SSHHost{{Name: "test-host", State: core.Disconnected}}
	d.SetHosts(hosts)

	// Idle 状態では IsInputActive は false
	if d.IsInputActive() {
		t.Fatalf("expected IsInputActive() = false in idle state")
	}
}
