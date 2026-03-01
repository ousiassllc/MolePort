package app

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

func TestMainModel_ConfigLoaded_ThemeUnset_ShowsThemePage(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	// 言語設定済み・テーマ未設定の ConfigLoadedMsg を送信
	msg := tui.ConfigLoadedMsg{ThemeBase: "", ThemeAccent: "", Language: "en"}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageTheme {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageTheme)
	}
	if !updated.isFirstLaunch {
		t.Error("isFirstLaunch should be true")
	}
	if updated.currentPresetID != theme.DefaultPresetID() {
		t.Errorf("currentPresetID = %q, want %q", updated.currentPresetID, theme.DefaultPresetID())
	}
}

func TestMainModel_ConfigLoaded_ThemeSet_AppliesTheme(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.ConfigLoadedMsg{ThemeBase: "dark", ThemeAccent: "blue", Language: "en"}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageDashboard {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageDashboard)
	}
	if updated.currentPresetID != "dark-blue" {
		t.Errorf("currentPresetID = %q, want %q", updated.currentPresetID, "dark-blue")
	}
}

func TestMainModel_ConfigLoaded_Error_LogsError(t *testing.T) {
	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.ConfigLoadedMsg{Err: fmt.Errorf("connection refused")}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	// エラー時はダッシュボードのまま
	if updated.currentPage != pageDashboard {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageDashboard)
	}
}

func TestMainModel_ThemeSelected_SwitchesToDashboard(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.currentPage = pageTheme
	m.isFirstLaunch = true

	msg := tui.ThemeSelectedMsg{PresetID: "dark-green"}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageDashboard {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageDashboard)
	}
	if updated.currentPresetID != "dark-green" {
		t.Errorf("currentPresetID = %q, want %q", updated.currentPresetID, "dark-green")
	}
	if updated.isFirstLaunch {
		t.Error("isFirstLaunch should be false after theme selection")
	}
	// saveTheme コマンドが返ることを確認（nil ではない）
	if cmd == nil {
		t.Error("ThemeSelectedMsg should return a save command")
	}
}

func TestMainModel_ThemeCancelled_FirstLaunch_AppliesDefault(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.currentPage = pageTheme
	m.isFirstLaunch = true

	msg := tui.ThemeCancelledMsg{}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageDashboard {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageDashboard)
	}
	if updated.currentPresetID != theme.DefaultPresetID() {
		t.Errorf("currentPresetID = %q, want %q", updated.currentPresetID, theme.DefaultPresetID())
	}
	if updated.isFirstLaunch {
		t.Error("isFirstLaunch should be false after cancel on first launch")
	}
	// 初回キャンセル時もデフォルトを保存するコマンドが返る
	if cmd == nil {
		t.Error("ThemeCancelledMsg on first launch should return a save command")
	}
}

func TestMainModel_ThemeCancelled_RestoresPrevious(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.currentPage = pageTheme
	m.isFirstLaunch = false
	m.previousPresetID = "dark-cyan"
	m.currentPresetID = "dark-orange" // テーマページでプレビュー中

	msg := tui.ThemeCancelledMsg{}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageDashboard {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageDashboard)
	}
	if updated.currentPresetID != "dark-cyan" {
		t.Errorf("currentPresetID = %q, want %q (previous)", updated.currentPresetID, "dark-cyan")
	}
	// 既存テーマに戻るだけなので保存コマンドは不要
	if cmd != nil {
		t.Error("ThemeCancelledMsg (not first launch) should not return a command")
	}
}

func TestMainModel_ThemeSavedMsg_Error_Logs(t *testing.T) {
	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.ThemeSavedMsg{Err: fmt.Errorf("save failed")}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if got := updated.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1 (error should be logged)", got)
	}
}

func TestMainModel_ThemeSavedMsg_Success_NoLog(t *testing.T) {
	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.ThemeSavedMsg{}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if got := updated.dashboard.LogLineCount(); got != 0 {
		t.Errorf("LogLineCount() = %d, want 0 (success should not log)", got)
	}
}
