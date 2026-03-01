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
	_ = i18n.SetLang(i18n.Lang(msg.Lang))
	m.currentLang = msg.Lang

	if m.isFirstLaunch {
		// 初回起動: 言語選択後にテーマ選択へ遷移
		m.currentPresetID = theme.DefaultPresetID()
		m.previousPresetID = m.currentPresetID
		m.themePage = pages.NewThemePage(m.currentPresetID)
		m.themePage.SetSize(m.width, m.height)
		m.currentPage = pageTheme
		return m, m.saveLang(msg.Lang)
	}

	// 通常の言語変更: ダッシュボードに戻る
	m.currentPage = pageDashboard
	return m, m.saveLang(msg.Lang)
}

// handleLangCancelled は言語キャンセルメッセージを処理する。
func (m MainModel) handleLangCancelled() (MainModel, tea.Cmd) {
	if m.isFirstLaunch {
		// 初回起動: デフォルト言語でテーマ選択へ遷移
		defaultLang := string(i18n.DefaultLang())
		_ = i18n.SetLang(i18n.DefaultLang())
		m.currentLang = defaultLang
		m.currentPresetID = theme.DefaultPresetID()
		m.previousPresetID = m.currentPresetID
		m.themePage = pages.NewThemePage(m.currentPresetID)
		m.themePage.SetSize(m.width, m.height)
		m.currentPage = pageTheme
		return m, m.saveLang(defaultLang)
	}

	// 通常の言語変更キャンセル: ダッシュボードに戻る
	m.currentPage = pageDashboard
	return m, nil
}

// handleLangSaved は言語保存完了メッセージを処理する。
func (m MainModel) handleLangSaved(msg tui.LangSavedMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil {
		m.dashboard.AppendLog("lang save error: " + msg.Err.Error())
	}
	return m, nil
}

// openLangPage は言語選択ページを開く。
func (m *MainModel) openLangPage() {
	m.langPage = pages.NewLangPage(m.currentLang)
	m.langPage.SetSize(m.width, m.height)
	m.currentPage = pageLang
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
