package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
	"github.com/ousiassllc/moleport/internal/tui/pages"
)

// DaemonManager はデーモンの起動・接続を抽象化するインターフェース。
// tui/app パッケージが daemon パッケージに直接依存しないようにする。
type DaemonManager interface {
	StartDaemonProcess(configDir string) (int, error)
	EnsureDaemonWithRetry(configDir string, maxWait time.Duration) (*client.IPCClient, error)
}

// MainModel はアプリケーションのルート Bubble Tea モデル。
type MainModel struct {
	dashboard      pages.DashboardPage
	client         *client.IPCClient
	daemonMgr      DaemonManager
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
	restarting         bool // デーモン再起動中フラグ

	// アップデート通知ダイアログ
	updateNotifyDialog molecules.InfoDialog
	showUpdateNotify   bool
	pendingUpdateCheck *tui.UpdateCheckDoneMsg

	// ヘルプモーダル
	showHelpModal bool

	// ページ遷移
	currentPage      string // "dashboard" | "theme" | "lang"
	themePage        pages.ThemePage
	langPage         pages.LangPage
	currentPresetID  string
	previousPresetID string
	currentLang      string
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

// SetDaemonManager はデーモン管理インターフェースを設定する。
// デーモン再起動機能を利用する場合に呼び出す。
func (m *MainModel) SetDaemonManager(dm DaemonManager) {
	m.daemonMgr = dm
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
		m.checkLatestVersion(),
	)
}

// Update は Bubble Tea の Update メソッド。
// メッセージをカテゴリ別のサブハンドラーに振り分ける。
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// 1. システムメッセージ（WindowSize, Key）
	if model, cmd, handled := m.handleSystemMsg(msg); handled {
		return model, cmd
	}

	// 2. UI 状態管理メッセージ（テーマ, 言語, バージョン, クレデンシャル等）
	if model, cmd, handled := m.handleUIMsg(msg); handled {
		return model, cmd
	}

	// 3. IPC 関連メッセージ（ホスト, セッション, 通知等）
	if model, cmd, handled := m.handleIPCMsg(msg); handled {
		return model, cmd
	}

	// 4. フォワード操作メッセージ
	if model, cmd, handled := m.handleForwardMsg(msg); handled {
		return model, cmd
	}

	// 未処理のメッセージはダッシュボードに転送
	var dashCmd tea.Cmd
	m.dashboard, dashCmd = m.dashboard.Update(msg)
	return m, dashCmd
}

// View は Bubble Tea の View メソッド。
func (m MainModel) View() string {
	if m.quitting {
		return i18n.T("tui.log.quitting") + "\n"
	}
	if m.showHelpModal {
		return m.renderHelpOverlay()
	}
	if m.showVersionConfirm {
		return m.renderVersionConfirmOverlay()
	}
	if m.showUpdateNotify {
		return m.renderUpdateNotifyOverlay()
	}
	if m.currentPage == pageTheme {
		return m.themePage.View()
	}
	if m.currentPage == pageLang {
		return m.langPage.View()
	}
	return m.dashboard.View()
}
