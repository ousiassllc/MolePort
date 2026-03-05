package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

func newTestModel(version string) MainModel {
	m := NewMainModel(nil, version, "/tmp/test")
	m.dashboard.SetSize(80, 24)
	return m
}

func TestVersionCheckDone(t *testing.T) {
	tests := []struct {
		name        string
		msg         tui.VersionCheckDoneMsg
		wantConfirm bool
		wantLogs    int
	}{
		{"match", tui.VersionCheckDoneMsg{Match: true}, false, 0},
		{"mismatch", tui.VersionCheckDoneMsg{DaemonVersion: "1.0.0", TUIVersion: "2.0.0"}, true, 0},
		{"error", tui.VersionCheckDoneMsg{Err: fmt.Errorf("connection refused")}, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := newTestModel("2.0.0").Update(tt.msg)
			u := result.(MainModel)
			if u.showVersionConfirm != tt.wantConfirm {
				t.Errorf("showVersionConfirm = %v, want %v", u.showVersionConfirm, tt.wantConfirm)
			}
			if got := u.dashboard.LogLineCount(); got != tt.wantLogs {
				t.Errorf("LogLineCount() = %d, want %d", got, tt.wantLogs)
			}
		})
	}
}

func TestVersionConfirmResult_No(t *testing.T) {
	m := newTestModel("2.0.0")
	m.showVersionConfirm = true
	result, cmd := m.Update(molecules.ConfirmResultMsg{Confirmed: false})
	u := result.(MainModel)
	if u.showVersionConfirm || cmd != nil {
		t.Error("showVersionConfirm should be false and cmd should be nil")
	}
	if got := u.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1", got)
	}
}

func TestVersionConfirmResult_Yes(t *testing.T) {
	m := NewMainModel(client.NewIPCClient("/tmp/nonexistent.sock"), "2.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.showVersionConfirm = true
	result, cmd := m.Update(molecules.ConfirmResultMsg{Confirmed: true})
	u := result.(MainModel)
	if u.showVersionConfirm || !u.restarting {
		t.Error("expected showVersionConfirm=false, restarting=true")
	}
	if cmd == nil {
		t.Error("restart command expected")
	}
}

func TestDaemonRestartDone(t *testing.T) {
	nc := client.NewIPCClient("/tmp/new.sock")
	tests := []struct {
		name    string
		msg     daemonRestartDoneMsg
		wantCmd bool
	}{
		{"error", daemonRestartDoneMsg{err: fmt.Errorf("failed")}, false},
		{"success", daemonRestartDoneMsg{newClient: nc}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMainModel(client.NewIPCClient("/tmp/old.sock"), "2.0.0", "/tmp/test")
			m.dashboard.SetSize(80, 24)
			m.restarting = true
			m.subscriptionID = "sub-123"
			result, cmd := m.Update(tt.msg)
			u := result.(MainModel)
			if u.restarting {
				t.Error("restarting should be false")
			}
			if (cmd != nil) != tt.wantCmd {
				t.Errorf("cmd nil=%v, wantCmd=%v", cmd == nil, tt.wantCmd)
			}
			if got := u.dashboard.LogLineCount(); got != 1 {
				t.Errorf("LogLineCount() = %d, want 1", got)
			}
		})
	}
	// success ケースでクライアントが入れ替わることを確認
	m := NewMainModel(client.NewIPCClient("/tmp/old.sock"), "2.0.0", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	result, _ := m.Update(daemonRestartDoneMsg{newClient: nc})
	u := result.(MainModel)
	if u.client != nc || u.subscriptionID != "" {
		t.Error("client should be replaced and subscriptionID reset")
	}
}

func TestRestartGuards(t *testing.T) {
	msgs := []struct {
		name string
		msg  any
	}{
		{"metrics_tick", tui.MetricsTickMsg{}},
		{"ipc_disconnected", tui.IPCDisconnectedMsg{}},
		{"log_output", tui.LogOutputMsg{Text: "error", Level: tui.LogError}},
		{"theme_saved_err", tui.ThemeSavedMsg{Err: fmt.Errorf("err")}},
	}
	for _, tt := range msgs {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel("1.0.0")
			m.restarting = true
			result, _ := m.Update(tt.msg)
			if got := result.(MainModel).dashboard.LogLineCount(); got != 0 {
				t.Errorf("LogLineCount() = %d, want 0", got)
			}
		})
	}
}

func TestView_ShowsDialogOverlays(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		m := newTestModel("2.0.0")
		m.width, m.height = 80, 24
		m.showVersionConfirm = true
		m.versionConfirm = molecules.NewConfirmDialog("バージョン不一致テスト")
		view := m.View()
		if !strings.Contains(view, "バージョン不一致テスト") {
			t.Error("View should contain confirm dialog message")
		}
	})
	t.Run("update_notify", func(t *testing.T) {
		m := newTestModel("1.0.0")
		m.width, m.height = 80, 24
		m.showUpdateNotify = true
		m.updateNotifyDialog = molecules.NewInfoDialog("MolePort 1.1.0 is available")
		if !strings.Contains(m.View(), "MolePort 1.1.0 is available") {
			t.Error("View should contain update notify dialog message")
		}
	})
}

func TestUpdateCheckDone(t *testing.T) {
	tests := []struct {
		name           string
		msg            tui.UpdateCheckDoneMsg
		versionConfirm bool
		wantDialog     bool
		wantPending    bool
	}{
		{"no_update", tui.UpdateCheckDoneMsg{UpdateAvailable: false}, false, false, false},
		{"update_available", tui.UpdateCheckDoneMsg{UpdateAvailable: true, CurrentVersion: "1.0.0", LatestVersion: "1.1.0"}, false, true, false},
		{"error_ignored", tui.UpdateCheckDoneMsg{Err: fmt.Errorf("error")}, false, false, false},
		{"buffered", tui.UpdateCheckDoneMsg{UpdateAvailable: true, CurrentVersion: "1.0.0", LatestVersion: "1.1.0"}, true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel("1.0.0")
			m.showVersionConfirm = tt.versionConfirm
			result, _ := m.Update(tt.msg)
			u := result.(MainModel)
			if u.showUpdateNotify != tt.wantDialog {
				t.Errorf("showUpdateNotify = %v, want %v", u.showUpdateNotify, tt.wantDialog)
			}
			if (u.pendingUpdateCheck != nil) != tt.wantPending {
				t.Errorf("pendingUpdateCheck nil=%v, wantPending=%v", u.pendingUpdateCheck == nil, tt.wantPending)
			}
		})
	}
}

func TestInfoDismissedMsg_ClosesDialog(t *testing.T) {
	m := newTestModel("1.0.0")
	m.showUpdateNotify = true
	m.updateNotifyDialog = molecules.NewInfoDialog("update available")
	result, _ := m.Update(molecules.InfoDismissedMsg{})
	if result.(MainModel).showUpdateNotify {
		t.Error("showUpdateNotify should be false")
	}
}

func TestVersionConfirmNo_ShowsPendingUpdate(t *testing.T) {
	m := newTestModel("1.0.0")
	m.showVersionConfirm = true
	m.pendingUpdateCheck = &tui.UpdateCheckDoneMsg{
		UpdateAvailable: true, CurrentVersion: "1.0.0", LatestVersion: "1.1.0",
	}
	result, _ := m.Update(molecules.ConfirmResultMsg{Confirmed: false})
	u := result.(MainModel)
	if u.showVersionConfirm || !u.showUpdateNotify || u.pendingUpdateCheck != nil {
		t.Error("expected showVersionConfirm=false, showUpdateNotify=true, pendingUpdateCheck=nil")
	}
}
