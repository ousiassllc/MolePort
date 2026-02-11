package app

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/pages"
)

// metricsInterval はメトリクス更新の間隔。
const metricsInterval = 2 * time.Second

// MainModel はアプリケーションのルート Bubble Tea モデル。
type MainModel struct {
	dashboard      pages.DashboardPage
	sshManager     core.SSHManager
	forwardManager core.ForwardManager
	configManager  core.ConfigManager
	keys           tui.KeyMap
	config         *core.Config
	hosts          []core.SSHHost
	quitting       bool
	sshSub         <-chan core.SSHEvent
	fwdSub         <-chan core.ForwardEvent
}

// NewMainModel は新しい MainModel を生成する。
func NewMainModel(
	sshMgr core.SSHManager,
	fwdMgr core.ForwardManager,
	cfgMgr core.ConfigManager,
) MainModel {
	return MainModel{
		dashboard:      pages.NewDashboardPage(),
		sshManager:     sshMgr,
		forwardManager: fwdMgr,
		configManager:  cfgMgr,
		keys:           tui.DefaultKeyMap(),
		config:         cfgMgr.GetConfig(),
	}
}

// Init は Bubble Tea の Init メソッド。初期読み込みコマンドを返す。
func (m MainModel) Init() tea.Cmd {
	m.sshSub = m.sshManager.Subscribe()
	m.fwdSub = m.forwardManager.Subscribe()

	return tea.Batch(
		m.loadHosts(),
		m.listenSSHEvents(),
		m.listenForwardEvents(),
		m.metricsTick(),
		m.dashboard.Init(),
	)
}

// Update は Bubble Tea の Update メソッド。
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.dashboard.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Ctrl+C は常にグローバル
		if key.Matches(msg, m.keys.ForceQuit) {
			return m, m.shutdown()
		}
		// テキスト入力中は q/? をグローバル処理しない
		if !m.dashboard.IsInputActive() {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, m.shutdown()
			case key.Matches(msg, m.keys.Help):
				m.showHelp()
				var cmd tea.Cmd
				m.dashboard, cmd = m.dashboard.Update(msg)
				return m, cmd
			}
		}

	case tui.HostsLoadedMsg:
		if msg.Err != nil {
			m.dashboard.AppendLog(fmt.Sprintf("ホスト読み込みエラー: %s", msg.Err))
		} else {
			m.hosts = msg.Hosts
			m.dashboard.SetHosts(msg.Hosts)
			m.refreshForwardPanel()
			m.dashboard.AppendLog(fmt.Sprintf("%d 件のホストを読み込みました", len(msg.Hosts)))

			// セッション自動復元
			if m.config.Session.AutoRestore {
				cmds = append(cmds, m.restoreSession())
			}
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

	case tui.SSHEventMsg:
		m.handleSSHEvent(msg.Event)
		cmds = append(cmds, m.listenSSHEvents())

	case tui.ForwardUpdatedMsg:
		m.refreshForwardPanel()
		cmds = append(cmds, m.listenForwardEvents())

	case tui.MetricsTickMsg:
		m.refreshForwardPanel()
		cmds = append(cmds, m.metricsTick())

	case tui.ForwardAddRequestMsg:
		cmd := m.handleForwardAdd(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tui.LogOutputMsg:
		m.dashboard.AppendLog(msg.Text)

	case tui.ForwardToggleMsg:
		cmds = append(cmds, m.toggleForward(msg.RuleName))

	case tui.ForwardDeleteRequestMsg:
		cmds = append(cmds, m.deleteForwardRule(msg.RuleName))

	case tui.ForwardDeleteConfirmedMsg:
		cmds = append(cmds, m.deleteForwardRule(msg.RuleName))

	case tui.SessionRestoredMsg:
		if msg.Err != nil {
			m.dashboard.AppendLog(fmt.Sprintf("セッション復元エラー: %s", msg.Err))
		} else {
			m.dashboard.AppendLog("セッションを復元しました")
		}
		m.refreshForwardPanel()

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
	return m.dashboard.View()
}

// --- フォワード操作 ---

func (m *MainModel) handleForwardAdd(msg tui.ForwardAddRequestMsg) tea.Cmd {
	remoteHost := msg.RemoteHost
	if remoteHost == "" && msg.Type != core.Dynamic {
		remoteHost = "localhost"
	}

	rule := core.ForwardRule{
		Name:        msg.Name,
		Host:        msg.Host,
		Type:        msg.Type,
		LocalPort:   msg.LocalPort,
		RemoteHost:  remoteHost,
		RemotePort:  msg.RemotePort,
		AutoConnect: msg.AutoConnect,
	}

	if err := m.forwardManager.AddRule(rule); err != nil {
		m.dashboard.AppendLog(fmt.Sprintf("ルール追加エラー: %s", err))
		return nil
	}

	m.dashboard.AppendLog(fmt.Sprintf("ルール '%s' を追加しました", rule.Name))
	m.refreshForwardPanel()
	m.saveForwardRules()

	if msg.AutoConnect {
		return m.startForward(rule.Name)
	}
	return nil
}

func (m *MainModel) deleteForwardRule(ruleName string) tea.Cmd {
	if err := m.forwardManager.DeleteRule(ruleName); err != nil {
		m.dashboard.AppendLog(fmt.Sprintf("ルール削除エラー: %s", err))
		return nil
	}
	m.dashboard.AppendLog(fmt.Sprintf("ルール '%s' を削除しました", ruleName))
	m.refreshForwardPanel()
	m.saveForwardRules()
	return nil
}

func (m *MainModel) toggleForward(ruleName string) tea.Cmd {
	session, err := m.forwardManager.GetSession(ruleName)
	if err != nil {
		m.dashboard.AppendLog(fmt.Sprintf("セッション取得エラー: %s", err))
		return nil
	}

	if session.Status == core.Active {
		return m.stopForward(ruleName)
	}
	return m.startForward(ruleName)
}

func (m *MainModel) startForward(ruleName string) tea.Cmd {
	return func() tea.Msg {
		if err := m.forwardManager.StartForward(ruleName); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード開始エラー (%s): %s", ruleName, err)}
		}
		return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード '%s' を開始しました", ruleName)}
	}
}

func (m *MainModel) stopForward(ruleName string) tea.Cmd {
	if err := m.forwardManager.StopForward(ruleName); err != nil {
		m.dashboard.AppendLog(fmt.Sprintf("フォワード停止エラー: %s", err))
		return nil
	}
	m.dashboard.AppendLog(fmt.Sprintf("フォワード '%s' を停止しました", ruleName))
	m.refreshForwardPanel()
	return nil
}

func (m *MainModel) showHelp() {
	m.dashboard.AppendLog("--- キー操作 ---")
	m.dashboard.AppendLog("  Tab         : ペイン切替 (Forwards ↔ Setup)")
	m.dashboard.AppendLog("  ↑/k ↓/j     : カーソル移動")
	m.dashboard.AppendLog("  Enter       : 選択 / 接続トグル")
	m.dashboard.AppendLog("  d           : 切断")
	m.dashboard.AppendLog("  x           : ルール削除")
	m.dashboard.AppendLog("  Esc         : ウィザードキャンセル")
	m.dashboard.AppendLog("  ?           : ヘルプ")
	m.dashboard.AppendLog("  q / Ctrl+C  : 終了")
}

func (m *MainModel) showList() {
	sessions := m.forwardManager.GetAllSessions()
	if len(sessions) == 0 {
		m.dashboard.AppendLog("フォワーディングルールがありません")
		return
	}

	m.dashboard.AppendLog("--- フォワーディングルール一覧 ---")
	for _, s := range sessions {
		status := s.Status.String()
		var desc string
		if s.Rule.Type == core.Dynamic {
			desc = fmt.Sprintf("  %s: %s :%d (SOCKS) [%s]", s.Rule.Name, s.Rule.Type, s.Rule.LocalPort, status)
		} else {
			desc = fmt.Sprintf("  %s: %s :%d -> %s:%d [%s]", s.Rule.Name, s.Rule.Type, s.Rule.LocalPort, s.Rule.RemoteHost, s.Rule.RemotePort, status)
		}
		m.dashboard.AppendLog(desc)
	}
}

func (m *MainModel) showStatus() {
	connectedCount := 0
	for _, h := range m.hosts {
		if h.State == core.Connected {
			connectedCount++
		}
	}

	sessions := m.forwardManager.GetAllSessions()
	activeCount := 0
	for _, s := range sessions {
		if s.Status == core.Active {
			activeCount++
		}
	}

	m.dashboard.AppendLog("--- ステータス ---")
	m.dashboard.AppendLog(fmt.Sprintf("  ホスト: %d (接続中: %d)", len(m.hosts), connectedCount))
	m.dashboard.AppendLog(fmt.Sprintf("  フォワード: %d (アクティブ: %d)", len(sessions), activeCount))
}

// --- 非同期コマンド ---

func (m *MainModel) loadHosts() tea.Cmd {
	return func() tea.Msg {
		hosts, err := m.sshManager.LoadHosts()
		return tui.HostsLoadedMsg{Hosts: hosts, Err: err}
	}
}

func (m *MainModel) listenSSHEvents() tea.Cmd {
	sub := m.sshSub
	return func() tea.Msg {
		event, ok := <-sub
		if !ok {
			return nil
		}
		return tui.SSHEventMsg{Event: event}
	}
}

func (m *MainModel) listenForwardEvents() tea.Cmd {
	sub := m.fwdSub
	return func() tea.Msg {
		event, ok := <-sub
		if !ok {
			return nil
		}
		return tui.ForwardUpdatedMsg{Event: event}
	}
}

func (m *MainModel) metricsTick() tea.Cmd {
	return tea.Tick(metricsInterval, func(time.Time) tea.Msg {
		return tui.MetricsTickMsg{}
	})
}

func (m *MainModel) restoreSession() tea.Cmd {
	return func() tea.Msg {
		state, err := m.configManager.LoadState()
		if err != nil {
			return tui.SessionRestoredMsg{Err: err}
		}
		if state == nil {
			return tui.SessionRestoredMsg{}
		}

		for _, rule := range state.ActiveForwards {
			_ = m.forwardManager.StartForward(rule.Name)
		}
		return tui.SessionRestoredMsg{}
	}
}

func (m *MainModel) shutdown() tea.Cmd {
	m.quitting = true

	// アクティブなフォワードの状態を保存
	sessions := m.forwardManager.GetAllSessions()
	var activeRules []core.ForwardRule
	for _, s := range sessions {
		if s.Status == core.Active {
			activeRules = append(activeRules, s.Rule)
		}
	}

	state := &core.State{
		LastUpdated:    time.Now(),
		ActiveForwards: activeRules,
	}
	_ = m.configManager.SaveState(state)

	// 全フォワードを停止
	m.forwardManager.Close()
	m.sshManager.Close()

	return tea.Quit
}

// --- ヘルパー ---

func (m *MainModel) handleSSHEvent(event core.SSHEvent) {
	m.dashboard.UpdateHostState(event.HostName, connectionStateFromEvent(event.Type))
	if event.Error != nil {
		m.dashboard.AppendLog(
			fmt.Sprintf("SSH [%s] %s: %s", event.HostName, event.Type, event.Error),
		)
	}
}

func connectionStateFromEvent(eventType core.SSHEventType) core.ConnectionState {
	switch eventType {
	case core.SSHEventConnected:
		return core.Connected
	case core.SSHEventDisconnected:
		return core.Disconnected
	case core.SSHEventReconnecting:
		return core.Reconnecting
	case core.SSHEventError:
		return core.ConnectionError
	default:
		return core.Disconnected
	}
}

func (m *MainModel) refreshForwardPanel() {
	sessions := m.forwardManager.GetAllSessions()
	m.dashboard.SetForwardSessions(sessions)
}

func (m *MainModel) saveForwardRules() {
	rules := m.forwardManager.GetRules()
	_ = m.configManager.UpdateConfig(func(c *core.Config) {
		c.Forwards = rules
	})
	m.config = m.configManager.GetConfig()
}
