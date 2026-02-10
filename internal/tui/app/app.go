package app

import (
	"fmt"
	"strconv"
	"strings"
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
	selectedHost   string
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
		// グローバルキー処理（ペインフォーカスに関わらず）
		if m.dashboard.FocusedPane() != tui.PaneCommand {
			switch {
			case key.Matches(msg, m.keys.ForceQuit):
				return m, m.shutdown()
			case key.Matches(msg, m.keys.Quit):
				return m, m.shutdown()
			case key.Matches(msg, m.keys.Help):
				m.showHelp()
				var cmd tea.Cmd
				m.dashboard, cmd = m.dashboard.Update(msg)
				return m, cmd
			}
		} else {
			// Command ペインでは Ctrl+C のみグローバル
			if key.Matches(msg, m.keys.ForceQuit) {
				return m, m.shutdown()
			}
		}

	case tui.HostsLoadedMsg:
		if msg.Err != nil {
			m.dashboard.AppendCommandOutput(fmt.Sprintf("ホスト読み込みエラー: %s", msg.Err))
		} else {
			m.hosts = msg.Hosts
			m.dashboard.SetHosts(msg.Hosts)
			if len(msg.Hosts) > 0 && m.selectedHost == "" {
				m.selectedHost = msg.Hosts[0].Name
				m.dashboard.SetSelectedHostName(m.selectedHost)
				m.refreshForwardPanel()
			}
			m.dashboard.AppendCommandOutput(fmt.Sprintf("%d 件のホストを読み込みました", len(msg.Hosts)))

			// セッション自動復元
			if m.config.Session.AutoRestore {
				cmds = append(cmds, m.restoreSession())
			}
		}

	case tui.HostsReloadedMsg:
		if msg.Err != nil {
			m.dashboard.AppendCommandOutput(fmt.Sprintf("ホスト再読み込みエラー: %s", msg.Err))
		} else {
			m.hosts = msg.Hosts
			m.dashboard.SetHosts(msg.Hosts)
			m.dashboard.AppendCommandOutput(fmt.Sprintf("%d 件のホストを再読み込みしました", len(msg.Hosts)))
		}

	case tui.HostSelectedMsg:
		m.selectedHost = msg.Host.Name
		m.dashboard.SetSelectedHostName(m.selectedHost)
		m.refreshForwardPanel()

	case tui.SSHEventMsg:
		m.handleSSHEvent(msg.Event)
		cmds = append(cmds, m.listenSSHEvents())

	case tui.ForwardUpdatedMsg:
		m.refreshForwardPanel()
		cmds = append(cmds, m.listenForwardEvents())

	case tui.MetricsTickMsg:
		m.refreshForwardPanel()
		cmds = append(cmds, m.metricsTick())

	case tui.CommandExecuteMsg:
		cmd := m.executeCommand(msg.Command, msg.Values)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tui.CommandOutputMsg:
		m.dashboard.AppendCommandOutput(msg.Text)

	case tui.ForwardToggleMsg:
		cmds = append(cmds, m.toggleForward(msg.RuleName))

	case tui.ForwardDeleteRequestMsg:
		m.dashboard.AppendCommandOutput(fmt.Sprintf("ルール '%s' を削除しますか？ (delete コマンドで確認)", msg.RuleName))

	case tui.ForwardDeleteConfirmedMsg:
		cmds = append(cmds, m.deleteForwardRule(msg.RuleName))

	case tui.SessionRestoredMsg:
		if msg.Err != nil {
			m.dashboard.AppendCommandOutput(fmt.Sprintf("セッション復元エラー: %s", msg.Err))
		} else {
			m.dashboard.AppendCommandOutput("セッションを復元しました")
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

// --- コマンド実行 ---

func (m *MainModel) executeCommand(command string, values map[string]string) tea.Cmd {
	switch command {
	case "add":
		return m.addForwardRule(values)
	case "delete":
		ruleName := values["rule_name"]
		return m.deleteForwardRule(ruleName)
	case "connect":
		ruleName := values["rule_name"]
		return m.startForward(ruleName)
	case "disconnect":
		ruleName := values["rule_name"]
		return m.stopForward(ruleName)
	case "reload":
		return m.reloadHosts()
	case "list":
		m.showList()
		return nil
	case "status":
		m.showStatus()
		return nil
	case "help":
		m.showHelp()
		return nil
	case "quit":
		return m.shutdown()
	case "config":
		m.handleConfigCommand(values)
		return nil
	default:
		m.dashboard.AppendCommandOutput(fmt.Sprintf("不明なコマンド: %s", command))
		return nil
	}
}

func (m *MainModel) addForwardRule(values map[string]string) tea.Cmd {
	host := values["host"]
	fwdType := values["type"]
	localPortStr := values["local_port"]
	remoteHost := values["remote_host"]
	remotePortStr := values["remote_port"]
	name := values["name"]
	autoConnect := strings.ToLower(values["auto_connect"]) == "y"

	localPort, err := strconv.Atoi(localPortStr)
	if err != nil {
		m.dashboard.AppendCommandOutput(fmt.Sprintf("ローカルポートエラー: %s", err))
		return nil
	}

	var remotePort int
	if remotePortStr != "" && remotePortStr != "0" {
		remotePort, err = strconv.Atoi(remotePortStr)
		if err != nil {
			m.dashboard.AppendCommandOutput(fmt.Sprintf("リモートポートエラー: %s", err))
			return nil
		}
	}

	parsedType, err := core.ParseForwardType(fwdType)
	if err != nil {
		m.dashboard.AppendCommandOutput(fmt.Sprintf("種別エラー: %s", err))
		return nil
	}

	if remoteHost == "" && parsedType != core.Dynamic {
		remoteHost = "localhost"
	}

	rule := core.ForwardRule{
		Name:        name,
		Host:        host,
		Type:        parsedType,
		LocalPort:   localPort,
		RemoteHost:  remoteHost,
		RemotePort:  remotePort,
		AutoConnect: autoConnect,
	}

	if err := m.forwardManager.AddRule(rule); err != nil {
		m.dashboard.AppendCommandOutput(fmt.Sprintf("ルール追加エラー: %s", err))
		return nil
	}

	m.dashboard.AppendCommandOutput(fmt.Sprintf("ルール '%s' を追加しました", rule.Name))
	m.refreshForwardPanel()
	m.saveForwardRules()

	if autoConnect {
		return m.startForward(rule.Name)
	}
	return nil
}

func (m *MainModel) deleteForwardRule(ruleName string) tea.Cmd {
	if err := m.forwardManager.DeleteRule(ruleName); err != nil {
		m.dashboard.AppendCommandOutput(fmt.Sprintf("ルール削除エラー: %s", err))
		return nil
	}
	m.dashboard.AppendCommandOutput(fmt.Sprintf("ルール '%s' を削除しました", ruleName))
	m.refreshForwardPanel()
	m.saveForwardRules()
	return nil
}

func (m *MainModel) toggleForward(ruleName string) tea.Cmd {
	session, err := m.forwardManager.GetSession(ruleName)
	if err != nil {
		m.dashboard.AppendCommandOutput(fmt.Sprintf("セッション取得エラー: %s", err))
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
			return tui.CommandOutputMsg{Text: fmt.Sprintf("フォワード開始エラー (%s): %s", ruleName, err)}
		}
		return tui.CommandOutputMsg{Text: fmt.Sprintf("フォワード '%s' を開始しました", ruleName)}
	}
}

func (m *MainModel) stopForward(ruleName string) tea.Cmd {
	if err := m.forwardManager.StopForward(ruleName); err != nil {
		m.dashboard.AppendCommandOutput(fmt.Sprintf("フォワード停止エラー: %s", err))
		return nil
	}
	m.dashboard.AppendCommandOutput(fmt.Sprintf("フォワード '%s' を停止しました", ruleName))
	m.refreshForwardPanel()
	return nil
}

func (m *MainModel) showList() {
	sessions := m.forwardManager.GetAllSessions()
	if len(sessions) == 0 {
		m.dashboard.AppendCommandOutput("フォワーディングルールがありません")
		return
	}

	m.dashboard.AppendCommandOutput("--- フォワーディングルール一覧 ---")
	for _, s := range sessions {
		status := s.Status.String()
		var desc string
		if s.Rule.Type == core.Dynamic {
			desc = fmt.Sprintf("  %s: %s :%d (SOCKS) [%s]", s.Rule.Name, s.Rule.Type, s.Rule.LocalPort, status)
		} else {
			desc = fmt.Sprintf("  %s: %s :%d -> %s:%d [%s]", s.Rule.Name, s.Rule.Type, s.Rule.LocalPort, s.Rule.RemoteHost, s.Rule.RemotePort, status)
		}
		m.dashboard.AppendCommandOutput(desc)
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

	m.dashboard.AppendCommandOutput("--- ステータス ---")
	m.dashboard.AppendCommandOutput(fmt.Sprintf("  ホスト: %d (接続中: %d)", len(m.hosts), connectedCount))
	m.dashboard.AppendCommandOutput(fmt.Sprintf("  フォワード: %d (アクティブ: %d)", len(sessions), activeCount))
}

func (m *MainModel) showHelp() {
	m.dashboard.AppendCommandOutput("--- コマンド一覧 ---")
	m.dashboard.AppendCommandOutput("  add (a)         : フォワードルール追加")
	m.dashboard.AppendCommandOutput("  delete (rm,del) : フォワードルール削除")
	m.dashboard.AppendCommandOutput("  connect (conn)  : フォワード接続開始")
	m.dashboard.AppendCommandOutput("  disconnect (dc) : フォワード切断")
	m.dashboard.AppendCommandOutput("  list (ls)       : ルール一覧表示")
	m.dashboard.AppendCommandOutput("  status (st)     : ステータス表示")
	m.dashboard.AppendCommandOutput("  reload (rl)     : ホスト一覧再読み込み")
	m.dashboard.AppendCommandOutput("  config (cfg)    : 設定変更")
	m.dashboard.AppendCommandOutput("  help (h)        : このヘルプ")
	m.dashboard.AppendCommandOutput("  quit (q)        : 終了")
}

func (m *MainModel) handleConfigCommand(values map[string]string) {
	category := values["category"]
	value := values["value"]

	switch category {
	case "reconnect":
		m.dashboard.AppendCommandOutput(fmt.Sprintf("reconnect 設定: enabled=%v, max_retries=%d",
			m.config.Reconnect.Enabled, m.config.Reconnect.MaxRetries))
		if value != "" {
			enabled := strings.ToLower(value) == "true" || strings.ToLower(value) == "on"
			if err := m.configManager.UpdateConfig(func(c *core.Config) {
				c.Reconnect.Enabled = enabled
			}); err != nil {
				m.dashboard.AppendCommandOutput(fmt.Sprintf("設定更新エラー: %s", err))
			} else {
				m.config = m.configManager.GetConfig()
				m.dashboard.AppendCommandOutput(fmt.Sprintf("reconnect.enabled = %v に更新しました", enabled))
			}
		}
	case "session":
		m.dashboard.AppendCommandOutput(fmt.Sprintf("session 設定: auto_restore=%v", m.config.Session.AutoRestore))
		if value != "" {
			autoRestore := strings.ToLower(value) == "true" || strings.ToLower(value) == "on"
			if err := m.configManager.UpdateConfig(func(c *core.Config) {
				c.Session.AutoRestore = autoRestore
			}); err != nil {
				m.dashboard.AppendCommandOutput(fmt.Sprintf("設定更新エラー: %s", err))
			} else {
				m.config = m.configManager.GetConfig()
				m.dashboard.AppendCommandOutput(fmt.Sprintf("session.auto_restore = %v に更新しました", autoRestore))
			}
		}
	case "log":
		m.dashboard.AppendCommandOutput(fmt.Sprintf("log 設定: level=%s, file=%s", m.config.Log.Level, m.config.Log.File))
		if value != "" {
			if err := m.configManager.UpdateConfig(func(c *core.Config) {
				c.Log.Level = value
			}); err != nil {
				m.dashboard.AppendCommandOutput(fmt.Sprintf("設定更新エラー: %s", err))
			} else {
				m.config = m.configManager.GetConfig()
				m.dashboard.AppendCommandOutput(fmt.Sprintf("log.level = %s に更新しました", value))
			}
		}
	default:
		m.dashboard.AppendCommandOutput("不明なカテゴリ: " + category)
	}
}

// --- 非同期コマンド ---

func (m *MainModel) loadHosts() tea.Cmd {
	return func() tea.Msg {
		hosts, err := m.sshManager.LoadHosts()
		return tui.HostsLoadedMsg{Hosts: hosts, Err: err}
	}
}

func (m *MainModel) reloadHosts() tea.Cmd {
	return func() tea.Msg {
		hosts, err := m.sshManager.ReloadHosts()
		return tui.HostsReloadedMsg{Hosts: hosts, Err: err}
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
		SelectedHost:   m.selectedHost,
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
		m.dashboard.AppendCommandOutput(
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
	var sessions []core.ForwardSession
	if m.selectedHost != "" {
		// 選択ホストのルールのみ表示
		rules := m.forwardManager.GetRulesByHost(m.selectedHost)
		for _, rule := range rules {
			session, err := m.forwardManager.GetSession(rule.Name)
			if err == nil {
				sessions = append(sessions, *session)
			}
		}
	} else {
		sessions = m.forwardManager.GetAllSessions()
	}
	m.dashboard.SetForwardSessions(sessions)
	m.dashboard.SetForwardRules(m.forwardManager.GetRules())
}

func (m *MainModel) saveForwardRules() {
	rules := m.forwardManager.GetRules()
	_ = m.configManager.UpdateConfig(func(c *core.Config) {
		c.Forwards = rules
	})
	m.config = m.configManager.GetConfig()
}
