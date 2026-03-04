package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// checkLatestVersion はデーモン経由で最新バージョンをチェックする Cmd を返す。
func (m *MainModel) checkLatestVersion() tea.Cmd {
	c, version := m.client, m.version
	return func() tea.Msg {
		if c == nil || version == "dev" {
			return tui.UpdateCheckDoneMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ipcReadTimeout)
		defer cancel()
		var result protocol.VersionCheckResult
		if err := c.Call(ctx, "version.check", protocol.VersionCheckParams{}, &result); err != nil {
			return tui.UpdateCheckDoneMsg{Err: err}
		}
		return tui.UpdateCheckDoneMsg{
			UpdateAvailable: result.UpdateAvailable,
			CurrentVersion:  result.CurrentVersion,
			LatestVersion:   result.LatestVersion,
			ReleaseURL:      result.ReleaseURL,
		}
	}
}

// handleUpdateCheckDone は最新バージョンチェック結果を処理する。
func (m MainModel) handleUpdateCheckDone(msg tui.UpdateCheckDoneMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil || !msg.UpdateAvailable {
		return m, nil
	}
	if m.showVersionConfirm {
		m.pendingUpdateCheck = &msg
		return m, nil
	}
	return m.showUpdateNotifyDialog(msg), nil
}

// showUpdateNotifyDialog はアップデート通知ダイアログを表示する。
func (m MainModel) showUpdateNotifyDialog(msg tui.UpdateCheckDoneMsg) MainModel {
	message := i18n.T("tui.update.available", map[string]any{
		"Latest": msg.LatestVersion, "Current": msg.CurrentVersion,
	})
	if msg.ReleaseURL != "" {
		message += "\n" + msg.ReleaseURL
	}
	m.updateNotifyDialog = molecules.NewInfoDialog(message)
	m.showUpdateNotify = true
	return m
}

// handleUpdateNotifyDismissed はアップデート通知ダイアログの閉じ処理を行う。
func (m MainModel) handleUpdateNotifyDismissed() (MainModel, tea.Cmd) {
	m.showUpdateNotify = false
	return m, nil
}

// renderUpdateNotifyOverlay はアップデート通知ダイアログのオーバーレイを描画する。
func (m MainModel) renderUpdateNotifyOverlay() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		m.updateNotifyDialog.View())
}
