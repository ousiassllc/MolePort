package app

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
)

func cleanupLang(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { _ = i18n.SetLang(i18n.DefaultLang()) })
}

func TestMainModel_ConfigLoaded_LangUnset(t *testing.T) {
	cleanupTheme(t)
	cleanupLang(t)
	u := updModel(newTestModel("test"), tui.ConfigLoadedMsg{})
	if u.currentPage != pageLang || !u.isFirstLaunch {
		t.Errorf("page=%q first=%v", u.currentPage, u.isFirstLaunch)
	}
}

func TestMainModel_LangSelected(t *testing.T) {
	t.Run("first_launch", func(t *testing.T) {
		cleanupTheme(t)
		cleanupLang(t)
		m := newTestModel("test")
		m.currentPage = pageLang
		m.isFirstLaunch = true
		result, cmd := m.Update(tui.LangSelectedMsg{Lang: "ja"})
		u := result.(MainModel)
		if u.currentPage != pageTheme || u.currentLang != "ja" || cmd == nil {
			t.Errorf("page=%q lang=%q cmd=%v", u.currentPage, u.currentLang, cmd)
		}
	})
	t.Run("normal", func(t *testing.T) {
		cleanupLang(t)
		m := newTestModel("test")
		m.currentPage = pageLang
		result, cmd := m.Update(tui.LangSelectedMsg{Lang: "en"})
		u := result.(MainModel)
		if u.currentPage != pageDashboard || u.currentLang != "en" || cmd == nil {
			t.Errorf("page=%q lang=%q cmd=%v", u.currentPage, u.currentLang, cmd)
		}
	})
}

func TestMainModel_LangCancelled(t *testing.T) {
	t.Run("first_launch", func(t *testing.T) {
		cleanupTheme(t)
		cleanupLang(t)
		m := newTestModel("test")
		m.currentPage = pageLang
		m.isFirstLaunch = true
		result, cmd := m.Update(tui.LangCancelledMsg{})
		u := result.(MainModel)
		if u.currentPage != pageTheme || u.currentLang != string(i18n.DefaultLang()) || cmd == nil {
			t.Errorf("page=%q lang=%q cmd=%v", u.currentPage, u.currentLang, cmd)
		}
	})
	t.Run("normal", func(t *testing.T) {
		cleanupLang(t)
		m := newTestModel("test")
		m.currentPage = pageLang
		result, cmd := m.Update(tui.LangCancelledMsg{})
		u := result.(MainModel)
		if u.currentPage != pageDashboard || cmd != nil {
			t.Errorf("page=%q cmd=%v", u.currentPage, cmd)
		}
	})
}

func TestMainModel_LangSavedMsg(t *testing.T) {
	u := updModel(newTestModel("test"), tui.LangSavedMsg{Err: fmt.Errorf("fail")})
	if got := u.dashboard.LogLineCount(); got != 1 {
		t.Errorf("error: LogLineCount() = %d, want 1", got)
	}
	u = updModel(newTestModel("test"), tui.LangSavedMsg{})
	if got := u.dashboard.LogLineCount(); got != 0 {
		t.Errorf("success: LogLineCount() = %d, want 0", got)
	}
}
