package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

func TestVersionCheckDone_Match_NoDialog(t *testing.T) {
	m := NewMainModel(nil, "1.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.VersionCheckDoneMsg{Match: true}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.showVersionConfirm {
		t.Error("showVersionConfirm should be false when versions match")
	}
	if cmd != nil {
		t.Error("no command should be returned when versions match")
	}
}

func TestVersionCheckDone_Mismatch_ShowsDialog(t *testing.T) {
	m := NewMainModel(nil, "2.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.VersionCheckDoneMsg{
		Match:         false,
		DaemonVersion: "1.0.0",
		TUIVersion:    "2.0.0",
	}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if !updated.showVersionConfirm {
		t.Error("showVersionConfirm should be true when versions mismatch")
	}
	if cmd != nil {
		t.Error("no command should be returned, only dialog shown")
	}
}

func TestVersionCheckDone_Error_LogsNoDialog(t *testing.T) {
	m := NewMainModel(nil, "1.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.VersionCheckDoneMsg{Err: fmt.Errorf("connection refused")}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if updated.showVersionConfirm {
		t.Error("showVersionConfirm should be false on error")
	}
	if got := updated.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1 (error should be logged)", got)
	}
}

func TestVersionConfirmResult_No_ShowsWarning(t *testing.T) {
	m := NewMainModel(nil, "2.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.showVersionConfirm = true

	msg := molecules.ConfirmResultMsg{Confirmed: false}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.showVersionConfirm {
		t.Error("showVersionConfirm should be false after confirm result")
	}
	if cmd != nil {
		t.Error("no command should be returned when user declines restart")
	}
	// ログにバージョン不一致の警告が出る
	if got := updated.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1 (warning should be logged)", got)
	}
}

func TestVersionConfirmResult_Yes_ReturnsRestartCmd(t *testing.T) {
	// restartDaemon は m.client を参照するため、ダミークライアントが必要
	dummyClient := client.NewIPCClient("/tmp/nonexistent.sock")
	m := NewMainModel(dummyClient, "2.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.showVersionConfirm = true

	msg := molecules.ConfirmResultMsg{Confirmed: true}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.showVersionConfirm {
		t.Error("showVersionConfirm should be false after confirm result")
	}
	if cmd == nil {
		t.Error("restart command should be returned when user confirms")
	}
	// ログに再起動中メッセージが出る
	if got := updated.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1 (restart message should be logged)", got)
	}
}

func TestDaemonRestartDone_Error_Logs(t *testing.T) {
	m := NewMainModel(nil, "2.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := daemonRestartDoneMsg{err: fmt.Errorf("failed to start daemon")}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if cmd != nil {
		t.Error("no command should be returned on restart error")
	}
	if got := updated.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1 (error should be logged)", got)
	}
}

func TestDaemonRestartDone_Success_ReplacesClient(t *testing.T) {
	oldClient := client.NewIPCClient("/tmp/old.sock")
	newClient := client.NewIPCClient("/tmp/new.sock")
	m := NewMainModel(oldClient, "2.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.subscriptionID = "sub-123"

	msg := daemonRestartDoneMsg{newClient: newClient}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.client != newClient {
		t.Error("client should be replaced with newClient")
	}
	if updated.subscriptionID != "" {
		t.Errorf("subscriptionID = %q, want empty (should be reset)", updated.subscriptionID)
	}
	if cmd == nil {
		t.Error("batch command should be returned to reload data")
	}
	if got := updated.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1 (success message should be logged)", got)
	}
}

func TestView_ShowsConfirmDialog_WhenVersionConfirmActive(t *testing.T) {
	m := NewMainModel(nil, "2.0.0", "/tmp/test")
	m.width = 80
	m.height = 24
	m.dashboard.SetSize(80, 24)
	m.showVersionConfirm = true
	m.versionConfirm = molecules.NewConfirmDialog("バージョン不一致テスト")

	view := m.View()

	if !strings.Contains(view, "バージョン不一致テスト") {
		t.Error("View should contain version confirm dialog message")
	}
	// ダッシュボードのヘッダーは表示されないこと
	if strings.Contains(view, "MolePort") {
		t.Error("View should NOT contain dashboard header when confirm dialog is shown")
	}
}
