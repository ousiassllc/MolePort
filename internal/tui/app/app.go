package app

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
	"github.com/ousiassllc/moleport/internal/tui/pages"
)

const (
	// metricsInterval はメトリクス更新の間隔。
	metricsInterval = 2 * time.Second
	// ipcReadTimeout は IPC 読み取り系操作のタイムアウト。
	ipcReadTimeout = 5 * time.Second
	// ipcWriteTimeout は IPC 書き込み系操作のタイムアウト。
	ipcWriteTimeout = 10 * time.Second
	// ipcShutdownTimeout はシャットダウン操作のタイムアウト。
	ipcShutdownTimeout = 2 * time.Second
)

// --- 内部メッセージ型 ---

type sessionsLoadedMsg struct {
	Sessions []core.ForwardSession
}

type subscriptionStartedMsg struct {
	SubscriptionID string
}

// MainModel はアプリケーションのルート Bubble Tea モデル。
type MainModel struct {
	dashboard      pages.DashboardPage
	client         *client.IPCClient
	keys           tui.KeyMap
	hosts          []core.SSHHost
	sessions       []core.ForwardSession
	quitting       bool
	subscriptionID string
	version        string
	configDir      string

	// クレデンシャル入力状態
	credRequest    *protocol.CredentialRequestNotification
	credResponseCh chan<- *protocol.CredentialResponseParams

	// バージョン確認ダイアログ
	versionConfirm     molecules.ConfirmDialog
	showVersionConfirm bool

	// テーマ / ページ遷移
	currentPage      string // "dashboard" | "theme"
	themePage        pages.ThemePage
	currentPresetID  string
	previousPresetID string
	isFirstLaunch    bool
	width            int
	height           int
}

// NewMainModel は新しい MainModel を生成する。
func NewMainModel(client *client.IPCClient, version string, configDir string) MainModel {
	return MainModel{
		dashboard:   pages.NewDashboardPage(version),
		client:      client,
		version:     version,
		configDir:   configDir,
		keys:        tui.DefaultKeyMap(),
		currentPage: pageDashboard,
	}
}

// Init は Bubble Tea の Init メソッド。初期読み込みコマンドを返す。
func (m MainModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadHosts(),
		m.loadSessions(),
		m.subscribeEvents(),
		m.metricsTick(),
		m.dashboard.Init(),
		m.loadConfig(),
		m.checkDaemonVersion(),
	)
}

// Update は Bubble Tea の Update メソッド。
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.dashboard.SetSize(msg.Width, msg.Height)
		m.themePage.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Ctrl+C は常にグローバル
		if key.Matches(msg, m.keys.ForceQuit) {
			return m, m.shutdown()
		}
		// バージョン確認ダイアログ表示中は ForceQuit 以外はダイアログに転送
		if m.showVersionConfirm {
			var cmd tea.Cmd
			m.versionConfirm, cmd = m.versionConfirm.Update(msg)
			return m, cmd
		}
		// テーマページ表示中は ForceQuit 以外は themePage に転送
		if m.currentPage == pageTheme {
			var cmd tea.Cmd
			m.themePage, cmd = m.themePage.Update(msg)
			return m, cmd
		}
		// テキスト入力中は q/?/t をグローバル処理しない
		if !m.dashboard.IsInputActive() {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, m.shutdown()
			case key.Matches(msg, m.keys.Help):
				m.showHelp()
				var cmd tea.Cmd
				m.dashboard, cmd = m.dashboard.Update(msg)
				return m, cmd
			case key.Matches(msg, m.keys.Theme):
				m.openThemePage()
				return m, nil
			}
		}

	case tui.VersionCheckDoneMsg:
		return m.handleVersionCheckDone(msg)

	case molecules.ConfirmResultMsg:
		if m.showVersionConfirm {
			return m.handleVersionConfirmResult(msg.Confirmed)
		}

	case daemonRestartDoneMsg:
		return m.handleDaemonRestartDone(msg)

	case tui.ConfigLoadedMsg:
		return m.handleConfigLoaded(msg)

	case tui.ThemeSelectedMsg:
		return m.handleThemeSelected(msg)

	case tui.ThemeCancelledMsg:
		return m.handleThemeCancelled()

	case tui.ThemeSavedMsg:
		return m.handleThemeSaved(msg)

	case tui.HostsLoadedMsg:
		if msg.Err != nil {
			m.dashboard.AppendLog(fmt.Sprintf("ホスト読み込みエラー: %s", msg.Err))
		} else {
			m.hosts = msg.Hosts
			m.dashboard.SetHosts(msg.Hosts)
			m.refreshForwardPanel()
			m.dashboard.AppendLog(fmt.Sprintf("%d 件のホストを読み込みました", len(msg.Hosts)))
		}

	case tui.HostsReloadedMsg:
		if msg.Err != nil {
			m.dashboard.AppendLog(fmt.Sprintf("ホスト再読み込みエラー: %s", msg.Err))
		} else {
			m.hosts = msg.Hosts
			m.dashboard.SetHosts(msg.Hosts)
			m.dashboard.AppendLog(fmt.Sprintf("%d 件のホストを再読み込みしました", len(msg.Hosts)))
		}

	case tui.HostSelectedMsg:
		// セットアップパネルが内部管理するため、ここでは何もしない

	case subscriptionStartedMsg:
		m.subscriptionID = msg.SubscriptionID
		cmds = append(cmds, m.listenIPCEvents())

	case sessionsLoadedMsg:
		m.sessions = msg.Sessions
		m.dashboard.SetForwardSessions(msg.Sessions)

	case tui.IPCNotificationMsg:
		m.handleIPCNotification(msg.Notification)
		cmds = append(cmds, m.listenIPCEvents())

	case tui.IPCDisconnectedMsg:
		m.dashboard.AppendLog("デーモンとの接続が切断されました")
		return m, m.shutdown()

	case tui.MetricsTickMsg:
		cmds = append(cmds, m.loadSessions())
		cmds = append(cmds, m.metricsTick())

	case tui.ForwardAddRequestMsg:
		cmd := m.handleForwardAdd(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tui.LogOutputMsg:
		m.dashboard.AppendLog(msg.Text)
		return m, nil

	case tui.ForwardToggleMsg:
		cmds = append(cmds, m.toggleForward(msg.RuleName))

	case tui.ForwardDeleteRequestMsg:
		cmds = append(cmds, m.deleteForwardRule(msg.RuleName))

	case tui.ForwardDeleteConfirmedMsg:
		cmds = append(cmds, m.deleteForwardRule(msg.RuleName))

	case tui.CredentialRequestMsg:
		return m.handleCredentialRequest(msg)

	case tui.CredentialSubmitMsg:
		return m.handleCredentialSubmit(msg)

	case tui.QuitRequestMsg:
		return m, m.shutdown()
	}

	// ダッシュボードにメッセージを送る
	var dashCmd tea.Cmd
	m.dashboard, dashCmd = m.dashboard.Update(msg)
	if dashCmd != nil {
		cmds = append(cmds, dashCmd)
	}

	return m, tea.Batch(cmds...)
}

// View は Bubble Tea の View メソッド。
func (m MainModel) View() string {
	if m.quitting {
		return "終了中...\n"
	}
	if m.showVersionConfirm {
		return m.renderVersionConfirmOverlay()
	}
	if m.currentPage == pageTheme {
		return m.themePage.View()
	}
	return m.dashboard.View()
}

// --- ヘルパー ---

func (m *MainModel) showHelp() {
	m.dashboard.AppendLog("--- キー操作 ---")
	m.dashboard.AppendLog("  Tab         : ペイン切替 (Forwards ↔ Setup)")
	m.dashboard.AppendLog("  /           : セットアップパネルにフォーカス")
	m.dashboard.AppendLog("  ↑/k ↓/j     : カーソル移動")
	m.dashboard.AppendLog("  Enter       : 選択 / 接続トグル")
	m.dashboard.AppendLog("  d           : 切断")
	m.dashboard.AppendLog("  x           : ルール削除")
	m.dashboard.AppendLog("  Esc         : ウィザードキャンセル")
	m.dashboard.AppendLog("  t           : テーマ選択")
	m.dashboard.AppendLog("  ?           : ヘルプ")
	m.dashboard.AppendLog("  q / Ctrl+C  : 終了")
}

func (m *MainModel) refreshForwardPanel() {
	m.dashboard.SetForwardSessions(m.sessions)
}

func (m *MainModel) shutdown() tea.Cmd {
	m.quitting = true
	// IPC クライアントをクリーンアップ（daemon は停止しない）
	if m.subscriptionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), ipcShutdownTimeout)
		defer cancel()
		_ = m.client.Unsubscribe(ctx, m.subscriptionID)
	}
	return tea.Quit
}
