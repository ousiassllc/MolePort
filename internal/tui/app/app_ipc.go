package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
)

// --- IPC 操作 ---

// loadHosts は host.list を呼んでホスト一覧を取得する。
func (m *MainModel) loadHosts() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcReadTimeout)
		defer cancel()
		var result protocol.HostListResult
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
		var result protocol.SessionListResult
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

func (m *MainModel) handleIPCNotification(notif *protocol.Notification) {
	switch notif.Method {
	case "event.ssh":
		var evt protocol.SSHEventNotification
		if err := json.Unmarshal(notif.Params, &evt); err != nil {
			return
		}
		state := parseConnectionState(evt.Type)
		m.dashboard.UpdateHostState(evt.Host, state)
		if evt.Error != "" {
			m.dashboard.AppendLog(fmt.Sprintf("SSH [%s] %s: %s", evt.Host, evt.Type, evt.Error))
		}
	case "event.forward":
		var evt protocol.ForwardEventNotification
		if err := json.Unmarshal(notif.Params, &evt); err != nil {
			return
		}
		m.dashboard.AppendLog(fmt.Sprintf("Forward [%s] %s", evt.Name, evt.Type))
		// セッション一覧は次の metricsTick で再読み込みされる
	}
}
