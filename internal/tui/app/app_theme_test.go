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
		if u.page.currentPage != pageTheme || !u.page.isFirstLaunch || u.page.currentPresetID != theme.DefaultPresetID() {
			t.Errorf("page=%q first=%v preset=%q", u.page.currentPage, u.page.isFirstLaunch, u.page.currentPresetID)
		}
	})
	t.Run("set_applies", func(t *testing.T) {
		cleanupTheme(t)
		u := updModel(newTestModel("test"), tui.ConfigLoadedMsg{ThemeBase: "dark", ThemeAccent: "blue", Language: "en"})
		if u.page.currentPage != pageDashboard || u.page.currentPresetID != "dark-blue" {
			t.Errorf("page=%q preset=%q", u.page.currentPage, u.page.currentPresetID)
		}
	})
	t.Run("error", func(t *testing.T) {
		u := updModel(newTestModel("test"), tui.ConfigLoadedMsg{Err: fmt.Errorf("err")})
		if u.page.currentPage != pageDashboard {
			t.Error("should remain pageDashboard on error")
		}
	})
}

func TestMainModel_ThemeSelected(t *testing.T) {
	cleanupTheme(t)
	m := newTestModel("test")
	m.page.currentPage = pageTheme
	m.page.isFirstLaunch = true
	result, cmd := m.Update(tui.ThemeSelectedMsg{PresetID: "dark-green"})
	u := result.(MainModel)
	if u.page.currentPage != pageDashboard || u.page.currentPresetID != "dark-green" || u.page.isFirstLaunch || cmd == nil {
		t.Errorf("page=%q preset=%q first=%v cmd=%v", u.page.currentPage, u.page.currentPresetID, u.page.isFirstLaunch, cmd)
	}
}

func TestMainModel_ThemeCancelled(t *testing.T) {
	t.Run("first_launch", func(t *testing.T) {
		cleanupTheme(t)
		m := newTestModel("test")
		m.page.currentPage = pageTheme
		m.page.isFirstLaunch = true
		result, cmd := m.Update(tui.ThemeCancelledMsg{})
		u := result.(MainModel)
		if u.page.currentPage != pageDashboard || u.page.currentPresetID != theme.DefaultPresetID() || u.page.isFirstLaunch || cmd == nil {
			t.Errorf("page=%q preset=%q first=%v cmd=%v", u.page.currentPage, u.page.currentPresetID, u.page.isFirstLaunch, cmd)
		}
	})
	t.Run("restores_previous", func(t *testing.T) {
		cleanupTheme(t)
		m := newTestModel("test")
		m.page.currentPage = pageTheme
		m.page.previousPresetID = "dark-cyan"
		m.page.currentPresetID = "dark-orange"
		result, cmd := m.Update(tui.ThemeCancelledMsg{})
		u := result.(MainModel)
		if u.page.currentPage != pageDashboard || u.page.currentPresetID != "dark-cyan" || cmd != nil {
			t.Errorf("page=%q preset=%q cmd=%v", u.page.currentPage, u.page.currentPresetID, cmd)
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
