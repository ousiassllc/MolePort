package app

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

func TestMainModel_ConfigLoaded_LangUnset_ShowsLangPage(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	t.Cleanup(func() { _ = i18n.SetLang(i18n.DefaultLang()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	// 言語未設定の ConfigLoadedMsg を送信
	msg := tui.ConfigLoadedMsg{ThemeBase: "", ThemeAccent: "", Language: ""}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageLang {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageLang)
	}
	if !updated.isFirstLaunch {
		t.Error("isFirstLaunch should be true")
	}
}

func TestMainModel_LangSelected_FirstLaunch_ShowsThemePage(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	t.Cleanup(func() { _ = i18n.SetLang(i18n.DefaultLang()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.currentPage = pageLang
	m.isFirstLaunch = true

	msg := tui.LangSelectedMsg{Lang: "ja"}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageTheme {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageTheme)
	}
	if updated.currentLang != "ja" {
		t.Errorf("currentLang = %q, want %q", updated.currentLang, "ja")
	}
	if cmd == nil {
		t.Error("LangSelectedMsg should return a save command")
	}
}

func TestMainModel_LangSelected_NormalMode_ReturnsToDashboard(t *testing.T) {
	t.Cleanup(func() { _ = i18n.SetLang(i18n.DefaultLang()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.currentPage = pageLang
	m.isFirstLaunch = false

	msg := tui.LangSelectedMsg{Lang: "en"}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageDashboard {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageDashboard)
	}
	if updated.currentLang != "en" {
		t.Errorf("currentLang = %q, want %q", updated.currentLang, "en")
	}
	if cmd == nil {
		t.Error("LangSelectedMsg should return a save command")
	}
}

func TestMainModel_LangCancelled_FirstLaunch_ShowsThemePage(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
	t.Cleanup(func() { _ = i18n.SetLang(i18n.DefaultLang()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.currentPage = pageLang
	m.isFirstLaunch = true

	msg := tui.LangCancelledMsg{}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageTheme {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageTheme)
	}
	if updated.currentLang != string(i18n.DefaultLang()) {
		t.Errorf("currentLang = %q, want %q", updated.currentLang, string(i18n.DefaultLang()))
	}
	if cmd == nil {
		t.Error("LangCancelledMsg on first launch should return a save command")
	}
}

func TestMainModel_LangCancelled_NormalMode_ReturnsToDashboard(t *testing.T) {
	t.Cleanup(func() { _ = i18n.SetLang(i18n.DefaultLang()) })

	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)
	m.currentPage = pageLang
	m.isFirstLaunch = false

	msg := tui.LangCancelledMsg{}
	result, cmd := m.Update(msg)
	updated := result.(MainModel)

	if updated.currentPage != pageDashboard {
		t.Errorf("currentPage = %q, want %q", updated.currentPage, pageDashboard)
	}
	if cmd != nil {
		t.Error("LangCancelledMsg (not first launch) should not return a command")
	}
}

func TestMainModel_LangSavedMsg_Error_Logs(t *testing.T) {
	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.LangSavedMsg{Err: fmt.Errorf("save failed")}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if got := updated.dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1 (error should be logged)", got)
	}
}

func TestMainModel_LangSavedMsg_Success_NoLog(t *testing.T) {
	m := NewMainModel(nil, "test", "/tmp/test")
	m.dashboard.SetSize(80, 24)

	msg := tui.LangSavedMsg{}
	result, _ := m.Update(msg)
	updated := result.(MainModel)

	if got := updated.dashboard.LogLineCount(); got != 0 {
		t.Errorf("LogLineCount() = %d, want 0 (success should not log)", got)
	}
}
