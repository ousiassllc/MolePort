package molecules

import (
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

// ---------------------------------------------------------------------------
// HostRow.View
// ---------------------------------------------------------------------------

func TestHostRow_View_ConnectedWithForwards(t *testing.T) {
	row := HostRow{
		Host: core.SSHHost{
			Name:               "webserver",
			HostName:           "192.168.1.10",
			Port:               22,
			User:               "admin",
			State:              core.Connected,
			ActiveForwardCount: 3,
		},
		Width: 120,
	}

	out := row.View()
	if !strings.Contains(out, "webserver") {
		t.Error("View() should contain host name 'webserver'")
	}
	if !strings.Contains(out, "admin") {
		t.Error("View() should contain user 'admin'")
	}
	if !strings.Contains(out, "3 fwd") {
		t.Error("View() should contain '3 fwd'")
	}
}

func TestHostRow_View_Disconnected(t *testing.T) {
	row := HostRow{
		Host: core.SSHHost{
			Name:     "dbserver",
			HostName: "10.0.0.5",
			Port:     22,
			User:     "root",
			State:    core.Disconnected,
		},
		Width: 120,
	}

	out := row.View()
	if !strings.Contains(out, "dbserver") {
		t.Error("View() should contain host name 'dbserver'")
	}
	if !strings.Contains(out, "0 fwd") {
		t.Error("View() should contain '0 fwd' for host with no forwards")
	}
}

// ---------------------------------------------------------------------------
// ConfirmDialog: Init / View
// ---------------------------------------------------------------------------

func TestConfirmDialog_Init(t *testing.T) {
	d := NewConfirmDialog("are you sure?")
	cmd := d.Init()
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestConfirmDialog_View(t *testing.T) {
	d := NewConfirmDialog("本当に削除しますか？")
	out := d.View()

	if !strings.Contains(out, "本当に削除しますか？") {
		t.Error("View() should contain the message text")
	}
	// i18n defaults to English; "Yes" and "No" are the translated values.
	if !strings.Contains(out, "Yes") {
		t.Error("View() should contain 'Yes' button text")
	}
	if !strings.Contains(out, "No") {
		t.Error("View() should contain 'No' button text")
	}
}

// ---------------------------------------------------------------------------
// InfoDialog: Init / View
// ---------------------------------------------------------------------------

func TestInfoDialog_Init(t *testing.T) {
	d := NewInfoDialog("info message")
	cmd := d.Init()
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestInfoDialog_View(t *testing.T) {
	d := NewInfoDialog("新しいバージョンがあります")
	out := d.View()

	if !strings.Contains(out, "新しいバージョンがあります") {
		t.Error("View() should contain the message text")
	}
	if !strings.Contains(out, "OK") {
		t.Error("View() should contain 'OK' button text")
	}
}

// ---------------------------------------------------------------------------
// PromptInput: Init / View
// ---------------------------------------------------------------------------

func TestPromptInput_Init(t *testing.T) {
	p := NewPromptInput()
	cmd := p.Init()
	if cmd == nil {
		t.Error("Init() should return textinput.Blink (non-nil)")
	}
}

func TestPromptInput_View(t *testing.T) {
	p := NewPromptInput()
	out := p.View()

	if out == "" {
		t.Fatal("View() should return non-empty string")
	}
	// The prompt contains ">" and placeholder "コマンドを入力..."
	if !strings.Contains(out, ">") {
		t.Error("View() should contain prompt character '>'")
	}
}
