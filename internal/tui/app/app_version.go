package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// daemonRestartDoneMsg はデーモン再起動完了を通知する内部メッセージ。
type daemonRestartDoneMsg struct {
	newClient *client.IPCClient
	err       error
}

// checkDaemonVersion はデーモンのバージョンを取得して TUI と比較する Cmd を返す。
// ゴルーチン安全のためクライアントポインタをローカル変数にキャプチャする。
func (m *MainModel) checkDaemonVersion() tea.Cmd {
	c := m.client // capture pointer for goroutine safety
	version := m.version
	return func() tea.Msg {
		if c == nil {
			return tui.VersionCheckDoneMsg{Match: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ipcReadTimeout)
		defer cancel()
		var status protocol.DaemonStatusResult
		if err := c.Call(ctx, "daemon.status", nil, &status); err != nil {
			return tui.VersionCheckDoneMsg{Err: err}
		}
		if status.Version == "dev" || version == "dev" {
			return tui.VersionCheckDoneMsg{Match: true}
		}
		return tui.VersionCheckDoneMsg{
			Match:         status.Version == version,
			DaemonVersion: status.Version,
			TUIVersion:    version,
		}
	}
}

// handleVersionCheckDone はバージョンチェック結果を処理する。
func (m MainModel) handleVersionCheckDone(msg tui.VersionCheckDoneMsg) (MainModel, tea.Cmd) {
	if msg.Err != nil {
		m.dashboard.AppendLog(i18n.T("tui.version.check_error", map[string]any{"Error": msg.Err}))
		return m, nil
	}
	if msg.Match {
		return m, nil
	}
	message := i18n.T("tui.version.mismatch", map[string]any{"DaemonVersion": msg.DaemonVersion, "TUIVersion": msg.TUIVersion})
	m.versionConfirm = molecules.NewConfirmDialog(message)
	m.showVersionConfirm = true
	return m, nil
}

// handleVersionConfirmResult はバージョン確認ダイアログの結果を処理する。
func (m MainModel) handleVersionConfirmResult(confirmed bool) (MainModel, tea.Cmd) {
	m.showVersionConfirm = false
	if confirmed {
		m.restarting = true
		m.dashboard.AppendLog(i18n.T("tui.version.restarting"))
		return m, m.restartDaemon()
	}
	m.dashboard.SetVersionWarning(true)
	m.dashboard.AppendLog(i18n.T("tui.version.mismatch_continue"))
	return m, nil
}

// restartDaemon はデーモンを再起動する Cmd を返す。
// ゴルーチン安全のため必要な値をすべてローカル変数にキャプチャする。
func (m *MainModel) restartDaemon() tea.Cmd {
	c := m.client                        // capture for goroutine
	configDir := m.configDir             // capture for goroutine
	credHandler := c.CredentialHandler() // save before shutdown
	return func() tea.Msg {
		// 1. デーモンをシャットダウン（失敗してもリスタートを続行する）
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var result protocol.DaemonShutdownResult
		if err := c.Call(ctx, "daemon.shutdown", protocol.DaemonShutdownParams{Purge: false}, &result); err != nil {
			slog.Warn("daemon shutdown failed, proceeding with restart", "error", err)
		}

		// 2. 旧接続を閉じる
		_ = c.Close()

		// 3. 新しいデーモンプロセスを起動
		if _, err := daemon.StartDaemonProcess(configDir); err != nil {
			return daemonRestartDoneMsg{err: fmt.Errorf("%s: %w", i18n.T("tui.log.daemon_start_failed"), err)}
		}

		// 4. 新しいデーモンに接続
		newClient, err := daemon.EnsureDaemonWithRetry(configDir, 5*time.Second)
		if err != nil {
			return daemonRestartDoneMsg{err: fmt.Errorf("%s: %w", i18n.T("tui.log.daemon_connect_failed"), err)}
		}

		// 5. クレデンシャルハンドラーを新しいクライアントに復元
		if credHandler != nil {
			newClient.SetCredentialHandler(credHandler)
		}

		return daemonRestartDoneMsg{newClient: newClient}
	}
}

// handleDaemonRestartDone はデーモン再起動完了を処理する。
// メインの Update ループで実行されるため、m.client の入れ替えはスレッドセーフ。
func (m MainModel) handleDaemonRestartDone(msg daemonRestartDoneMsg) (MainModel, tea.Cmd) {
	m.restarting = false
	if msg.err != nil {
		m.dashboard.AppendLog(i18n.T("tui.version.restart_error", map[string]any{"Error": msg.err}))
		return m, nil
	}
	m.client = msg.newClient
	m.subscriptionID = ""
	m.dashboard.AppendLog(i18n.T("tui.version.restarted"))
	return m, tea.Batch(
		m.loadHosts(),
		m.loadSessions(),
		m.subscribeEvents(),
		m.loadConfig(),
	)
}

// renderVersionConfirmOverlay はバージョン確認ダイアログのオーバーレイを描画する。
func (m MainModel) renderVersionConfirmOverlay() string {
	dialog := m.versionConfirm.View()
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}
