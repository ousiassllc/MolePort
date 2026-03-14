package setuppanel

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

func makeHosts(names ...string) []core.SSHHost {
	var hosts []core.SSHHost
	for _, n := range names {
		hosts = append(hosts, core.SSHHost{Name: n, HostName: n + ".example.com"})
	}
	return hosts
}

func typeRunes(p Panel, s string) Panel {
	for _, r := range s {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return p
}

func setupWizardAt(step WizardStep) Panel {
	p := New()
	p.focused = true
	p.hosts = []core.SSHHost{{Name: "test-host", User: "user", HostName: "example.com", Port: 22}}
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	p, _ = p.Update(enter) // -> StepSelectType
	if step == StepSelectType {
		return p
	}
	p, _ = p.Update(enter) // -> StepLocalPort (Local type)
	return p
}

func TestPanel_SetHosts(t *testing.T) {
	p := New()
	p.SetHosts(makeHosts("alpha", "beta"))
	if got := p.Hosts(); len(got) != 2 || got[0].Name != "alpha" {
		t.Errorf("Hosts roundtrip failed: len=%d", len(p.Hosts()))
	}
	// Cursor adjusts on shrink
	p.SetHosts(makeHosts("a", "b", "c"))
	p.hostCursor = 2
	p.SetHosts(makeHosts("a"))
	if p.hostCursor != 0 {
		t.Errorf("cursor after shrink=%d want 0", p.hostCursor)
	}
	p.hostCursor = 5
	p.SetHosts(nil)
	if p.hostCursor != 0 {
		t.Errorf("cursor after empty=%d want 0", p.hostCursor)
	}
}

func TestPanel_SetFocused_And_SetSize(t *testing.T) {
	p := New()
	p.SetFocused(true)
	if !p.focused {
		t.Error("SetFocused(true) failed")
	}
	p.SetFocused(false)
	if p.focused {
		t.Error("SetFocused(false) failed")
	}
	p.SetSize(80, 24)
	if p.width != 80 || p.height != 24 {
		t.Errorf("SetSize: got %dx%d", p.width, p.height)
	}
}

func TestPanel_IsInputActive(t *testing.T) {
	p := New()
	for _, step := range []WizardStep{StepIdle, StepSelectType, StepConfirm} {
		p.step = step
		if p.IsInputActive() {
			t.Errorf("step %d should be false", step)
		}
	}
	for _, step := range []WizardStep{StepLocalPort, StepRemoteHost, StepRemotePort, StepRuleName} {
		p.step = step
		if !p.IsInputActive() {
			t.Errorf("step %d should be true", step)
		}
	}
}

func TestPanel_UpdateHostState(t *testing.T) {
	p := New()
	p.SetHosts(makeHosts("s1", "s2"))
	p.UpdateHostState("s2", core.Connected)
	if p.Hosts()[1].State != core.Connected || p.Hosts()[0].State != core.Disconnected {
		t.Error("UpdateHostState did not update correctly")
	}
	p.UpdateHostState("nonexistent", core.Connected) // should not panic
}
func TestPanel_Update_NotFocused(t *testing.T) {
	p := New()
	p.SetHosts(makeHosts("a"))
	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || p2.step != StepIdle {
		t.Error("Update when not focused should be noop")
	}
}

func TestValidatePortStr(t *testing.T) {
	for _, tt := range []struct {
		in      string
		wantErr bool
	}{
		{"8080", false}, {"1", false}, {"65535", false},
		{"", true}, {"abc", true}, {"0", true}, {"65536", true}, {"-1", true},
	} {
		if err := validatePortStr(tt.in); (err != nil) != tt.wantErr {
			t.Errorf("validatePortStr(%q)=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
	}
}

func TestPanel_View_And_EscReset(t *testing.T) {
	p := New()
	p.focused = true
	p.hosts = []core.SSHHost{{Name: "h1", User: "u", HostName: "h1.example.com", Port: 22}}
	p.SetSize(60, 20)
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	for _, step := range []string{"Idle", "SelectType", "LocalPort"} {
		if p.View() == "" {
			t.Errorf("View at %s should be non-empty", step)
		}
		if step != "LocalPort" {
			p, _ = p.Update(enter)
		}
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.step != StepIdle {
		t.Errorf("Esc should reset to StepIdle, got %d", p.step)
	}
}

func TestPanel_TextInputKeystrokes(t *testing.T) {
	// Port input
	p := setupWizardAt(StepLocalPort)
	p = typeRunes(p, "8080")
	if got := p.portInput.Value(); got != "8080" {
		t.Errorf("portInput=%q want 8080", got)
	}
	// Host input
	p = setupWizardAt(StepLocalPort)
	p = typeRunes(p, "3000")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.step != StepRemoteHost {
		t.Fatalf("expected StepRemoteHost, got %d", p.step)
	}
	p = typeRunes(p, "db.local")
	if got := p.hostInput.Value(); got != "db.local" {
		t.Errorf("hostInput=%q want db.local", got)
	}
}

func TestPanel_UpdateIdle_CursorAndSelect(t *testing.T) {
	p := New()
	p.focused = true
	p.SetHosts(makeHosts("h1", "h2", "h3"))
	p.SetSize(60, 20)
	down, up := tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyUp}
	p, cmd := p.Update(down)
	if p.hostCursor != 1 || cmd == nil {
		t.Errorf("down: cursor=%d cmd=%v", p.hostCursor, cmd)
	}
	p, _ = p.Update(up)
	p, _ = p.Update(up) // clamp at top
	if p.hostCursor != 0 {
		t.Errorf("up clamp: cursor=%d want 0", p.hostCursor)
	}
	for range 4 {
		p, _ = p.Update(down)
	}
	if p.hostCursor != 2 {
		t.Errorf("down clamp: cursor=%d want 2", p.hostCursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.step != StepSelectType || p.selectedHost != "h3" {
		t.Errorf("enter: step=%d host=%q", p.step, p.selectedHost)
	}
}

func TestPanel_UpdateSelectType_AllTypes(t *testing.T) {
	for _, tt := range []struct {
		downs    int
		wantType core.ForwardType
	}{
		{0, core.Local}, {1, core.Remote}, {2, core.Dynamic},
	} {
		p := setupWizardAt(StepSelectType)
		for range tt.downs {
			p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if p.selectedType != tt.wantType || p.step != StepLocalPort {
			t.Errorf("downs=%d: type=%v step=%d", tt.downs, p.selectedType, p.step)
		}
	}
}

func TestPanel_AdvanceFromTextStep_FullWizard(t *testing.T) {
	p := setupWizardAt(StepLocalPort)
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	p = typeRunes(p, "3000")
	p, _ = p.Update(enter) // -> RemoteHost
	if p.step != StepRemoteHost {
		t.Fatalf("after localPort: step=%d", p.step)
	}
	p, _ = p.Update(enter) // empty host -> RemotePort
	if p.step != StepRemotePort || p.remoteHost != "localhost" {
		t.Fatalf("after remoteHost: step=%d host=%q", p.step, p.remoteHost)
	}
	p = typeRunes(p, "80")
	p, _ = p.Update(enter) // -> RuleName
	if p.step != StepRuleName {
		t.Fatalf("after remotePort: step=%d", p.step)
	}
	p, _ = p.Update(enter) // empty name -> Confirm
	if p.step != StepConfirm {
		t.Fatalf("after ruleName: step=%d", p.step)
	}
}

func TestPanel_AdvanceFromTextStep_DynamicSkipsRemote(t *testing.T) {
	p := setupWizardAt(StepSelectType)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Dynamic
	p = typeRunes(p, "1080")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.step != StepRuleName {
		t.Errorf("dynamic after localPort: step=%d want StepRuleName", p.step)
	}
}

func TestPanel_PlaceholderAutofill(t *testing.T) {
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	t.Run("EmptyEnterLocalPort", func(t *testing.T) {
		p := setupWizardAt(StepLocalPort)
		p, _ = p.Update(enter) // 空 Enter → placeholder "8080"
		if p.step != StepRemoteHost {
			t.Fatalf("step=%d want StepRemoteHost", p.step)
		}
		if p.localPort != "8080" {
			t.Errorf("localPort=%q want 8080", p.localPort)
		}
	})
	t.Run("EmptyEnterRemotePort_and_Placeholder", func(t *testing.T) {
		p := setupWizardAt(StepLocalPort)
		p = typeRunes(p, "3000")
		p, _ = p.Update(enter) // -> RemoteHost
		p, _ = p.Update(enter) // -> RemotePort
		if p.portInput.Placeholder != "3000" {
			t.Errorf("placeholder=%q want 3000", p.portInput.Placeholder)
		}
		p, _ = p.Update(enter) // 空 Enter → placeholder "3000"
		if p.step != StepRuleName {
			t.Fatalf("step=%d want StepRuleName", p.step)
		}
		if p.remotePort != "3000" {
			t.Errorf("remotePort=%q want 3000", p.remotePort)
		}
	})
}

func TestPanel_AdvanceFromTextStep_InvalidPort(t *testing.T) {
	p := setupWizardAt(StepLocalPort)
	p = typeRunes(p, "abc")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.step != StepLocalPort {
		t.Errorf("invalid port should stay at StepLocalPort, got %d", p.step)
	}
}

func TestPanel_UpdateConfirm(t *testing.T) {
	p := setupWizardAt(StepLocalPort)
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	p = typeRunes(p, "8080")
	p, _ = p.Update(enter) // -> RemoteHost
	p, _ = p.Update(enter) // -> RemotePort (host=localhost)
	p = typeRunes(p, "80")
	p, _ = p.Update(enter) // -> RuleName
	p, _ = p.Update(enter) // -> Confirm
	if p.step != StepConfirm {
		t.Fatalf("expected StepConfirm, got %d", p.step)
	}
	p, cmd := p.Update(enter)
	if cmd == nil {
		t.Fatal("confirm Enter should produce cmd")
	}
	msg, ok := cmd().(tui.ForwardAddRequestMsg)
	if !ok {
		t.Fatalf("expected ForwardAddRequestMsg, got %T", cmd())
	}
	if msg.LocalPort != 8080 || msg.RemotePort != 80 || msg.RemoteHost != "localhost" {
		t.Errorf("msg: local=%d remote=%d host=%q", msg.LocalPort, msg.RemotePort, msg.RemoteHost)
	}
	if p.step != StepIdle {
		t.Errorf("after confirm: step=%d want StepIdle", p.step)
	}
}

func TestPanel_ViewConfirm(t *testing.T) {
	for _, tt := range []struct {
		name string
		typ  core.ForwardType
	}{
		{"Local", core.Local}, {"Dynamic", core.Dynamic},
	} {
		p := New()
		p.focused = true
		p.SetSize(60, 20)
		p.step = StepConfirm
		p.selectedType = tt.typ
		p.localPort = "8080"
		p.remoteHost = "localhost"
		p.remotePort = "80"
		p.ruleName = "r"
		if p.View() == "" {
			t.Errorf("viewConfirm %s should produce non-empty output", tt.name)
		}
	}
}
