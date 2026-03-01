package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/pages"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

// ページ識別子の定数。
const (
	pageDashboard = "dashboard"
	pageTheme     = "theme"
)

// handleConfigLoaded は設定読み込み完了メッセージを処理する。
func (m MainModel) handleConfigLoaded(msg tui.ConfigLoadedMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil {
		m.dashboard.AppendLog("config load error: " + msg.Err.Error())
		return m, nil
	}
	if msg.ThemeBase == "" || msg.ThemeAccent == "" {
		m.isFirstLaunch = true
		m.currentPresetID = theme.DefaultPresetID()
		m.previousPresetID = m.currentPresetID
		m.themePage = pages.NewThemePage(m.currentPresetID)
		m.themePage.SetSize(m.width, m.height)
		m.currentPage = pageTheme
	} else {
		presetID := theme.PresetIDFromConfig(msg.ThemeBase, msg.ThemeAccent)
		theme.Apply(presetID)
		m.currentPresetID = presetID
	}
	return m, nil
}

// handleThemeSelected はテーマ選択メッセージを処理する。
func (m MainModel) handleThemeSelected(msg tui.ThemeSelectedMsg) (MainModel, tea.Cmd) {
	theme.Apply(msg.PresetID)
	m.currentPresetID = msg.PresetID
	m.currentPage = pageDashboard
	m.isFirstLaunch = false
	return m, m.saveTheme(msg.PresetID)
}

// handleThemeCancelled はテーマキャンセルメッセージを処理する。
func (m MainModel) handleThemeCancelled() (MainModel, tea.Cmd) {
	if m.isFirstLaunch {
		defaultID := theme.DefaultPresetID()
		theme.Apply(defaultID)
		m.currentPresetID = defaultID
		m.currentPage = pageDashboard
		m.isFirstLaunch = false
		return m, m.saveTheme(defaultID)
	}
	theme.Apply(m.previousPresetID)
	m.currentPresetID = m.previousPresetID
	m.currentPage = pageDashboard
	return m, nil
}

// handleThemeSaved はテーマ保存完了メッセージを処理する。
func (m MainModel) handleThemeSaved(msg tui.ThemeSavedMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil {
		m.dashboard.AppendLog("theme save error: " + msg.Err.Error())
	}
	return m, nil
}

// openThemePage はテーマ選択ページを開く。
func (m *MainModel) openThemePage() {
	m.previousPresetID = m.currentPresetID
	m.themePage = pages.NewThemePage(m.currentPresetID)
	m.themePage.SetSize(m.width, m.height)
	m.currentPage = pageTheme
}
