package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/pages"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

// ページ識別子の定数。
const (
	pageDashboard = "dashboard"
	pageTheme     = "theme"
	pageLang      = "lang"
)

// handleConfigLoaded は設定読み込み完了メッセージを処理する。
func (m MainModel) handleConfigLoaded(msg tui.ConfigLoadedMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil {
		if !m.dialog.restarting {
			m.dashboard.AppendLog(i18n.T("tui.log.config_load_error", map[string]any{"Error": msg.Err}), tui.LogError)
		}
		return m, nil
	}

	// 言語が未設定 → 初回起動: 言語選択ページから開始
	if msg.Language == "" {
		m.page.isFirstLaunch = true
		m.openLangPage()
		return m, nil
	}

	// 言語が設定済み → 適用
	_ = i18n.SetLang(i18n.Lang(msg.Language)) // ベストエフォート: 未知の言語でもフォールバックされる
	m.page.currentLang = msg.Language

	// テーマが未設定 → テーマ選択ページへ
	if msg.ThemeBase == "" || msg.ThemeAccent == "" {
		m.page.isFirstLaunch = true
		m.page.currentPresetID = theme.DefaultPresetID()
		m.page.previousPresetID = m.page.currentPresetID
		m.page.themePage = pages.NewThemePage(m.page.currentPresetID)
		m.page.themePage.SetSize(m.width, m.height)
		m.page.currentPage = pageTheme
	} else {
		presetID := theme.PresetIDFromConfig(msg.ThemeBase, msg.ThemeAccent)
		theme.Apply(presetID)
		m.page.currentPresetID = presetID
	}
	return m, nil
}

// handleThemeSelected はテーマ選択メッセージを処理する。
func (m MainModel) handleThemeSelected(msg tui.ThemeSelectedMsg) (MainModel, tea.Cmd) {
	theme.Apply(msg.PresetID)
	m.page.currentPresetID = msg.PresetID
	m.page.currentPage = pageDashboard
	m.page.isFirstLaunch = false
	return m, m.saveTheme(msg.PresetID)
}

// handleThemeCancelled はテーマキャンセルメッセージを処理する。
func (m MainModel) handleThemeCancelled() (MainModel, tea.Cmd) {
	if m.page.isFirstLaunch {
		defaultID := theme.DefaultPresetID()
		theme.Apply(defaultID)
		m.page.currentPresetID = defaultID
		m.page.currentPage = pageDashboard
		m.page.isFirstLaunch = false
		return m, m.saveTheme(defaultID)
	}
	theme.Apply(m.page.previousPresetID)
	m.page.currentPresetID = m.page.previousPresetID
	m.page.currentPage = pageDashboard
	return m, nil
}

// handleThemeSaved はテーマ保存完了メッセージを処理する。
func (m MainModel) handleThemeSaved(msg tui.ThemeSavedMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil && !m.dialog.restarting {
		m.dashboard.AppendLog(i18n.T("tui.log.theme_save_error", map[string]any{"Error": msg.Err}), tui.LogError)
	}
	return m, nil
}

// openThemePage はテーマ選択ページを開く。
func (m *MainModel) openThemePage() {
	m.page.previousPresetID = m.page.currentPresetID
	m.page.themePage = pages.NewThemePage(m.page.currentPresetID)
	m.page.themePage.SetSize(m.width, m.height)
	m.page.currentPage = pageTheme
}
