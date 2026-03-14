package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/pages"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

// handleLangSelected は言語選択メッセージを処理する。
func (m MainModel) handleLangSelected(msg tui.LangSelectedMsg) (MainModel, tea.Cmd) {
	_ = i18n.SetLang(i18n.Lang(msg.Lang)) // ベストエフォート: 未知の言語でもフォールバックされる
	m.page.currentLang = msg.Lang

	if m.page.isFirstLaunch {
		// 初回起動: 言語選択後にテーマ選択へ遷移
		m.page.currentPresetID = theme.DefaultPresetID()
		m.page.previousPresetID = m.page.currentPresetID
		m.page.themePage = pages.NewThemePage(m.page.currentPresetID)
		m.page.themePage.SetSize(m.width, m.height)
		m.page.currentPage = pageTheme
		return m, m.saveLang(msg.Lang)
	}

	// 通常の言語変更: ダッシュボードに戻る
	m.page.currentPage = pageDashboard
	return m, m.saveLang(msg.Lang)
}

// handleLangCancelled は言語キャンセルメッセージを処理する。
func (m MainModel) handleLangCancelled() (MainModel, tea.Cmd) {
	if m.page.isFirstLaunch {
		// 初回起動: デフォルト言語でテーマ選択へ遷移
		defaultLang := string(i18n.DefaultLang())
		_ = i18n.SetLang(i18n.DefaultLang()) // ベストエフォート: デフォルト言語は常に成功する
		m.page.currentLang = defaultLang
		m.page.currentPresetID = theme.DefaultPresetID()
		m.page.previousPresetID = m.page.currentPresetID
		m.page.themePage = pages.NewThemePage(m.page.currentPresetID)
		m.page.themePage.SetSize(m.width, m.height)
		m.page.currentPage = pageTheme
		return m, m.saveLang(defaultLang)
	}

	// 通常の言語変更キャンセル: ダッシュボードに戻る
	m.page.currentPage = pageDashboard
	return m, nil
}

// handleLangSaved は言語保存完了メッセージを処理する。
func (m MainModel) handleLangSaved(msg tui.LangSavedMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil && !m.dialog.restarting {
		m.dashboard.AppendLog(i18n.T("tui.log.lang_save_error", map[string]any{"Error": msg.Err}), tui.LogError)
	}
	return m, nil
}

// openLangPage は言語選択ページを開く。
func (m *MainModel) openLangPage() {
	m.page.langPage = pages.NewLangPage(m.page.currentLang)
	m.page.langPage.SetSize(m.width, m.height)
	m.page.currentPage = pageLang
}

// saveLang は config.update で言語設定を保存する。
func (m *MainModel) saveLang(lang string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := protocol.ConfigUpdateParams{
			Language: &lang,
		}
		var result protocol.ConfigUpdateResult
		if err := c.Call(ctx, "config.update", params, &result); err != nil {
			return tui.LangSavedMsg{Err: fmt.Errorf("config.update: %w", err)}
		}
		return tui.LangSavedMsg{}
	}
}
