package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
	"github.com/ousiassllc/moleport/internal/tui"
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
	client         *ipc.IPCClient
	keys           tui.KeyMap
	hosts          []core.SSHHost
	sessions       []core.ForwardSession
	quitting       bool
	subscriptionID string
}

// NewMainModel は新しい MainModel を生成する。
func NewMainModel(client *ipc.IPCClient, version string) MainModel {
	return MainModel{
		dashboard: pages.NewDashboardPage(version),
		client:    client,
		keys:      tui.DefaultKeyMap(),
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

	case tui.ForwardToggleMsg:
		cmds = append(cmds, m.toggleForward(msg.RuleName))

	case tui.ForwardDeleteRequestMsg:
		cmds = append(cmds, m.deleteForwardRule(msg.RuleName))

	case tui.ForwardDeleteConfirmedMsg:
		cmds = append(cmds, m.deleteForwardRule(msg.RuleName))

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

// --- IPC 操作 ---

// loadHosts は host.list を呼んでホスト一覧を取得する。
func (m *MainModel) loadHosts() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcReadTimeout)
		defer cancel()
		var result ipc.HostListResult
		if err := m.client.Call(ctx, "host.list", nil, &result); err != nil {
			return tui.HostsLoadedMsg{Err: err}
		}
		hosts := make([]core.SSHHost, len(result.Hosts))
		for i, h := range result.Hosts {
			hosts[i] = hostInfoToSSHHost(h)
		}
		return tui.HostsLoadedMsg{Hosts: hosts}
	}
}

// loadSessions は session.list を呼んでセッション一覧を取得する。
func (m *MainModel) loadSessions() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcReadTimeout)
		defer cancel()
		var result ipc.SessionListResult
		if err := m.client.Call(ctx, "session.list", nil, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("セッション取得エラー: %s", err)}
		}
		sessions := make([]core.ForwardSession, len(result.Sessions))
		for i, s := range result.Sessions {
			sessions[i] = sessionInfoToForwardSession(s)
		}
		return sessionsLoadedMsg{Sessions: sessions}
	}
}

// subscribeEvents はイベント購読を開始する。
func (m *MainModel) subscribeEvents() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcReadTimeout)
		defer cancel()
		subID, err := m.client.Subscribe(ctx, []string{"ssh", "forward"})
		if err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("イベント購読エラー: %s", err)}
		}
		return subscriptionStartedMsg{SubscriptionID: subID}
	}
}

// listenIPCEvents は IPC イベントチャネルから次の通知を受信する。
func (m *MainModel) listenIPCEvents() tea.Cmd {
	events := m.client.Events()
	return func() tea.Msg {
		notif, ok := <-events
		if !ok {
			return tui.IPCDisconnectedMsg{}
		}
		return tui.IPCNotificationMsg{Notification: notif}
	}
}

func (m *MainModel) metricsTick() tea.Cmd {
	return tea.Tick(metricsInterval, func(time.Time) tea.Msg {
		return tui.MetricsTickMsg{}
	})
}

// --- IPC 通知ハンドリング ---

func (m *MainModel) handleIPCNotification(notif *ipc.Notification) {
	switch notif.Method {
	case "event.ssh":
		var evt ipc.SSHEventNotification
		if err := json.Unmarshal(notif.Params, &evt); err != nil {
			return
		}
		state := parseConnectionState(evt.Type)
		m.dashboard.UpdateHostState(evt.Host, state)
		if evt.Error != "" {
			m.dashboard.AppendLog(fmt.Sprintf("SSH [%s] %s: %s", evt.Host, evt.Type, evt.Error))
		}
	case "event.forward":
		var evt ipc.ForwardEventNotification
		if err := json.Unmarshal(notif.Params, &evt); err != nil {
			return
		}
		m.dashboard.AppendLog(fmt.Sprintf("Forward [%s] %s", evt.Name, evt.Type))
		// セッション一覧は次の metricsTick で再読み込みされる
	}
}

// --- フォワード操作 ---

func (m *MainModel) handleForwardAdd(msg tui.ForwardAddRequestMsg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := ipc.ForwardAddParams{
			Name:        msg.Name,
			Host:        msg.Host,
			Type:        msg.Type.String(),
			LocalPort:   msg.LocalPort,
			RemoteHost:  msg.RemoteHost,
			RemotePort:  msg.RemotePort,
			AutoConnect: msg.AutoConnect,
		}
		var result ipc.ForwardAddResult
		if err := m.client.Call(ctx, "forward.add", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("ルール追加エラー: %s", err)}
		}

		// AutoConnect が設定されている場合はフォワードも開始
		if msg.AutoConnect {
			startParams := ipc.ForwardStartParams{Name: result.Name}
			var startResult ipc.ForwardStartResult
			if err := m.client.Call(ctx, "forward.start", startParams, &startResult); err != nil {
				return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' を追加しましたが、開始に失敗: %s", result.Name, err)}
			}
			return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' を追加し、開始しました", result.Name)}
		}

		return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' を追加しました", result.Name)}
	}
}

func (m *MainModel) deleteForwardRule(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := ipc.ForwardDeleteParams{Name: ruleName}
		var result ipc.ForwardDeleteResult
		if err := m.client.Call(ctx, "forward.delete", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("ルール削除エラー: %s", err)}
		}
		return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' を削除しました", ruleName)}
	}
}

func (m *MainModel) toggleForward(ruleName string) tea.Cmd {
	// ローカルのセッション情報から状態を判定する
	for _, s := range m.sessions {
		if s.Rule.Name == ruleName {
			if s.Status == core.Active {
				return m.stopForward(ruleName)
			}
			return m.startForward(ruleName)
		}
	}
	return m.startForward(ruleName)
}

func (m *MainModel) startForward(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := ipc.ForwardStartParams{Name: ruleName}
		var result ipc.ForwardStartResult
		if err := m.client.Call(ctx, "forward.start", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード開始エラー (%s): %s", ruleName, err)}
		}
		return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード '%s' を開始しました", ruleName)}
	}
}

func (m *MainModel) stopForward(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := ipc.ForwardStopParams{Name: ruleName}
		var result ipc.ForwardStopResult
		if err := m.client.Call(ctx, "forward.stop", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード停止エラー: %s", err)}
		}
		return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード '%s' を停止しました", ruleName)}
	}
}

// --- ヘルパー ---

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
