package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// handleSystemMsg は tea.WindowSizeMsg と tea.KeyMsg を処理する。
// 処理した場合は handled=true を返す。
func (m MainModel) handleSystemMsg(msg tea.Msg) (MainModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.dashboard.SetSize(msg.Width, msg.Height)
		m.themePage.SetSize(msg.Width, msg.Height)
		m.langPage.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd, true

	case tea.KeyMsg:
		model, cmd, handled := m.handleKeyMsg(msg)
		return model, cmd, handled
	}
	return m, nil, false
}

// handleKeyMsg はキーメッセージを処理する。
func (m MainModel) handleKeyMsg(msg tea.KeyMsg) (MainModel, tea.Cmd, bool) {
	// Ctrl+C は常にグローバル
	if key.Matches(msg, m.keys.ForceQuit) {
		return m, m.shutdown(), true
	}
	// ヘルプモーダル表示中は任意のキーで閉じる
	if m.showHelpModal {
		m.showHelpModal = false
		return m, nil, true
	}
	// アップデート通知ダイアログ表示中は ForceQuit 以外はダイアログに転送
	// showUpdateNotify と showVersionConfirm は相互排他（handleUpdateCheckDone でバッファリング）
	if m.showUpdateNotify {
		var cmd tea.Cmd
		m.updateNotifyDialog, cmd = m.updateNotifyDialog.Update(msg)
		return m, cmd, true
	}
	// バージョン確認ダイアログ表示中は ForceQuit 以外はダイアログに転送
	if m.showVersionConfirm {
		var cmd tea.Cmd
		m.versionConfirm, cmd = m.versionConfirm.Update(msg)
		return m, cmd, true
	}
	// テーマページ表示中は ForceQuit 以外は themePage に転送
	if m.currentPage == pageTheme {
		var cmd tea.Cmd
		m.themePage, cmd = m.themePage.Update(msg)
		return m, cmd, true
	}
	// 言語ページ表示中は ForceQuit 以外は langPage に転送
	if m.currentPage == pageLang {
		var cmd tea.Cmd
		m.langPage, cmd = m.langPage.Update(msg)
		return m, cmd, true
	}
	// テキスト入力中は q/?/t/l をグローバル処理しない
	if !m.dashboard.IsInputActive() {
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, m.shutdown(), true
		case key.Matches(msg, m.keys.Help):
			m.showHelpModal = true
			return m, nil, true
		case key.Matches(msg, m.keys.Theme):
			m.openThemePage()
			return m, nil, true
		case key.Matches(msg, m.keys.Lang):
			m.openLangPage()
			return m, nil, true
		case key.Matches(msg, m.keys.Version):
			m.dashboard.AppendLog(fmt.Sprintf("MolePort %s", m.version), tui.LogInfo)
			return m, nil, true
		}
	}
	return m, nil, false
}

// handleIPCMsg は IPC 関連のメッセージを処理する。
// 処理した場合は handled=true を返す。cmds はフォールスルーに利用。
func (m MainModel) handleIPCMsg(msg tea.Msg) (MainModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tui.HostsLoadedMsg:
		if msg.Err != nil {
			if !m.restarting {
				m.dashboard.AppendLog(i18n.T("tui.log.hosts_load_error", map[string]any{"Error": msg.Err}), tui.LogError)
			}
		} else {
			m.hosts = msg.Hosts
			m.dashboard.SetHosts(msg.Hosts)
			m.refreshForwardPanel()
			m.dashboard.AppendLog(i18n.T("tui.log.hosts_loaded", map[string]any{"Count": len(msg.Hosts)}), tui.LogSuccess)
		}
		return m, nil, true

	case tui.HostsReloadedMsg:
		if msg.Err != nil {
			m.dashboard.AppendLog(i18n.T("tui.log.hosts_reload_error", map[string]any{"Error": msg.Err}), tui.LogError)
		} else {
			m.hosts = msg.Hosts
			m.dashboard.SetHosts(msg.Hosts)
			m.dashboard.AppendLog(i18n.T("tui.log.hosts_reloaded", map[string]any{"Count": len(msg.Hosts)}), tui.LogSuccess)
		}
		return m, nil, true

	case tui.HostSelectedMsg:
		// セットアップパネルが内部管理するため、ここでは何もしない
		return m, nil, true

	case subscriptionStartedMsg:
		m.subscriptionID = msg.SubscriptionID
		return m, m.listenIPCEvents(), true

	case sessionsLoadedMsg:
		m.sessions = msg.Sessions
		m.dashboard.SetForwardSessions(msg.Sessions)
		return m, nil, true

	case tui.IPCNotificationMsg:
		m.handleIPCNotification(msg.Notification)
		return m, m.listenIPCEvents(), true

	case tui.IPCDisconnectedMsg:
		if m.restarting {
			return m, nil, true
		}
		m.dashboard.AppendLog(i18n.T("tui.log.daemon_disconnected"), tui.LogError)
		return m, m.shutdown(), true

	case tui.MetricsTickMsg:
		var cmds []tea.Cmd
		if !m.restarting {
			cmds = append(cmds, m.loadSessions())
		}
		cmds = append(cmds, m.metricsTick())
		return m, tea.Batch(cmds...), true
	}
	return m, nil, false
}

// handleForwardMsg はフォワード操作関連のメッセージを処理する。
// 処理した場合は handled=true を返す。
func (m MainModel) handleForwardMsg(msg tea.Msg) (MainModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tui.ForwardAddRequestMsg:
		cmd := m.handleForwardAdd(msg)
		return m, cmd, true

	case tui.ForwardToggleMsg:
		return m, m.toggleForward(msg.RuleName), true

	case tui.ForwardDeleteRequestMsg:
		return m, m.deleteForwardRule(msg.RuleName), true

	case tui.ForwardDeleteConfirmedMsg:
		return m, m.deleteForwardRule(msg.RuleName), true

	case tui.LogOutputMsg:
		if !m.restarting {
			m.dashboard.AppendLog(msg.Text, msg.Level)
		}
		return m, nil, true
	}
	return m, nil, false
}

// handleUIMsg は UI 状態管理関連のメッセージを処理する。
// 処理した場合は handled=true を返す。
func (m MainModel) handleUIMsg(msg tea.Msg) (MainModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tui.VersionCheckDoneMsg:
		model, cmd := m.handleVersionCheckDone(msg)
		return model, cmd, true

	case tui.UpdateCheckDoneMsg:
		model, cmd := m.handleUpdateCheckDone(msg)
		return model, cmd, true

	case molecules.InfoDismissedMsg:
		if m.showUpdateNotify {
			model, cmd := m.handleUpdateNotifyDismissed()
			return model, cmd, true
		}
		return m, nil, true

	case molecules.ConfirmResultMsg:
		if m.showVersionConfirm {
			model, cmd := m.handleVersionConfirmResult(msg.Confirmed)
			return model, cmd, true
		}
		return m, nil, true

	case daemonRestartDoneMsg:
		model, cmd := m.handleDaemonRestartDone(msg)
		return model, cmd, true

	case tui.ConfigLoadedMsg:
		model, cmd := m.handleConfigLoaded(msg)
		return model, cmd, true

	case tui.ThemeSelectedMsg:
		model, cmd := m.handleThemeSelected(msg)
		return model, cmd, true

	case tui.ThemeCancelledMsg:
		model, cmd := m.handleThemeCancelled()
		return model, cmd, true

	case tui.ThemeSavedMsg:
		model, cmd := m.handleThemeSaved(msg)
		return model, cmd, true

	case tui.LangSelectedMsg:
		model, cmd := m.handleLangSelected(msg)
		return model, cmd, true

	case tui.LangCancelledMsg:
		model, cmd := m.handleLangCancelled()
		return model, cmd, true

	case tui.LangSavedMsg:
		model, cmd := m.handleLangSaved(msg)
		return model, cmd, true

	case tui.CredentialRequestMsg:
		model, cmd := m.handleCredentialRequest(msg)
		return model.(MainModel), cmd, true

	case tui.CredentialSubmitMsg:
		model, cmd := m.handleCredentialSubmit(msg)
		return model.(MainModel), cmd, true

	case tui.QuitRequestMsg:
		return m, m.shutdown(), true
	}
	return m, nil, false
}
