package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

const (
	// metricsInterval はメトリクス更新の間隔。
	metricsInterval = 2 * time.Second
	// ipcReadTimeout は IPC 読み取り系操作のタイムアウト。
	ipcReadTimeout = 5 * time.Second
	// ipcWriteTimeout は IPC 書き込み系操作のタイムアウト。
	ipcWriteTimeout = 10 * time.Second
	// ipcCredentialTimeout はクレデンシャル待ちを含む操作のタイムアウト。
	ipcCredentialTimeout = 40 * time.Second
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
			return tui.LogOutputMsg{Text: i18n.T("tui.log.session_error", map[string]any{"Error": err})}
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
			return tui.LogOutputMsg{Text: i18n.T("tui.log.subscribe_error", map[string]any{"Error": err})}
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

// loadConfig は config.get を呼んでテーマ設定を取得する。
func (m *MainModel) loadConfig() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcReadTimeout)
		defer cancel()
		var result protocol.ConfigGetResult
		if err := m.client.Call(ctx, "config.get", nil, &result); err != nil {
			return tui.ConfigLoadedMsg{Err: err}
		}
		return tui.ConfigLoadedMsg{
			ThemeBase:   result.TUI.Theme.Base,
			ThemeAccent: result.TUI.Theme.Accent,
			Language:    result.Language,
		}
	}
}

// saveTheme は config.update でテーマ設定を保存する。
func (m *MainModel) saveTheme(presetID string) tea.Cmd {
	return func() tea.Msg {
		p, ok := theme.FindPreset(presetID)
		if !ok {
			return tui.ThemeSavedMsg{Err: fmt.Errorf("unknown preset: %s", presetID)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		base := p.Base
		accent := p.Accent
		params := protocol.ConfigUpdateParams{
			TUI: &protocol.TUIUpdateInfo{
				Theme: &protocol.ThemeUpdateInfo{
					Base:   &base,
					Accent: &accent,
				},
			},
		}
		var result protocol.ConfigUpdateResult
		if err := m.client.Call(ctx, "config.update", params, &result); err != nil {
			return tui.ThemeSavedMsg{Err: err}
		}
		return tui.ThemeSavedMsg{}
	}
}

// --- IPC 通知ハンドリング ---

func (m *MainModel) handleIPCNotification(notif *protocol.Notification) {
	switch notif.Method {
	case "event.ssh":
		var evt protocol.SSHEventNotification
		if err := json.Unmarshal(notif.Params, &evt); err != nil {
			slog.Warn("failed to unmarshal notification", "method", notif.Method, "error", err)
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
			slog.Warn("failed to unmarshal notification", "method", notif.Method, "error", err)
			return
		}
		m.dashboard.AppendLog(fmt.Sprintf("Forward [%s] %s", evt.Name, evt.Type))
		// セッション一覧は次の metricsTick で再読み込みされる
	}
}
