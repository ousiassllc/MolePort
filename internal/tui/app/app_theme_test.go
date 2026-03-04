package app

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

func cleanupTheme(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { theme.Apply(theme.DefaultPresetID()) })
}

func TestMainModel_ConfigLoaded_Theme(t *testing.T) {
	t.Run("unset_shows_theme_page", func(t *testing.T) {
		cleanupTheme(t)
		u := updModel(newTestModel("test"), tui.ConfigLoadedMsg{Language: "en"})
		if u.currentPage != pageTheme || !u.isFirstLaunch || u.currentPresetID != theme.DefaultPresetID() {
			t.Errorf("page=%q first=%v preset=%q", u.currentPage, u.isFirstLaunch, u.currentPresetID)
		}
	})
	t.Run("set_applies", func(t *testing.T) {
		cleanupTheme(t)
		u := updModel(newTestModel("test"), tui.ConfigLoadedMsg{ThemeBase: "dark", ThemeAccent: "blue", Language: "en"})
		if u.currentPage != pageDashboard || u.currentPresetID != "dark-blue" {
			t.Errorf("page=%q preset=%q", u.currentPage, u.currentPresetID)
		}
	})
	t.Run("error", func(t *testing.T) {
		u := updModel(newTestModel("test"), tui.ConfigLoadedMsg{Err: fmt.Errorf("err")})
		if u.currentPage != pageDashboard {
			t.Error("should remain pageDashboard on error")
		}
	})
}

func TestMainModel_ThemeSelected(t *testing.T) {
	cleanupTheme(t)
	m := newTestModel("test")
	m.currentPage = pageTheme
	m.isFirstLaunch = true
	result, cmd := m.Update(tui.ThemeSelectedMsg{PresetID: "dark-green"})
	u := result.(MainModel)
	if u.currentPage != pageDashboard || u.currentPresetID != "dark-green" || u.isFirstLaunch || cmd == nil {
		t.Errorf("page=%q preset=%q first=%v cmd=%v", u.currentPage, u.currentPresetID, u.isFirstLaunch, cmd)
	}
}

func TestMainModel_ThemeCancelled(t *testing.T) {
	t.Run("first_launch", func(t *testing.T) {
		cleanupTheme(t)
		m := newTestModel("test")
		m.currentPage = pageTheme
		m.isFirstLaunch = true
		result, cmd := m.Update(tui.ThemeCancelledMsg{})
		u := result.(MainModel)
		if u.currentPage != pageDashboard || u.currentPresetID != theme.DefaultPresetID() || u.isFirstLaunch || cmd == nil {
			t.Errorf("page=%q preset=%q first=%v cmd=%v", u.currentPage, u.currentPresetID, u.isFirstLaunch, cmd)
		}
	})
	t.Run("restores_previous", func(t *testing.T) {
		cleanupTheme(t)
		m := newTestModel("test")
		m.currentPage = pageTheme
		m.previousPresetID = "dark-cyan"
		m.currentPresetID = "dark-orange"
		result, cmd := m.Update(tui.ThemeCancelledMsg{})
		u := result.(MainModel)
		if u.currentPage != pageDashboard || u.currentPresetID != "dark-cyan" || cmd != nil {
			t.Errorf("page=%q preset=%q cmd=%v", u.currentPage, u.currentPresetID, cmd)
		}
	})
}

func TestMainModel_ThemeSavedMsg(t *testing.T) {
	u := updModel(newTestModel("test"), tui.ThemeSavedMsg{Err: fmt.Errorf("fail")})
	if got := u.dashboard.LogLineCount(); got != 1 {
		t.Errorf("error: LogLineCount() = %d, want 1", got)
	}
	u = updModel(newTestModel("test"), tui.ThemeSavedMsg{})
	if got := u.dashboard.LogLineCount(); got != 0 {
		t.Errorf("success: LogLineCount() = %d, want 0", got)
	}
}
