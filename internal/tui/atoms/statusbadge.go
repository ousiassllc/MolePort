package atoms

import (
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

// 接続状態に対応するシンボルの定義
var connectionSymbols = map[core.ConnectionState]string{
	core.Connected:       "●",
	core.Disconnected:    "○",
	core.ConnectionError: "✗",
	core.Reconnecting:    "◌",
	core.Connecting:      "◌",
	core.PendingAuth:     "◎",
}

// セッション状態に対応するシンボルの定義
var sessionSymbols = map[core.SessionStatus]string{
	core.Active:              "●",
	core.Stopped:             "○",
	core.SessionError:        "✗",
	core.SessionReconnecting: "◌",
	core.Starting:            "◌",
}

// RenderConnectionBadge は SSH 接続状態をカラーシンボルとして描画する（シンボルのみ）。
func RenderConnectionBadge(state core.ConnectionState) string {
	symbol, ok := connectionSymbols[state]
	if !ok {
		return tui.MutedStyle().Render("?")
	}
	switch state {
	case core.Connected:
		return tui.ActiveStyle().Render(symbol)
	case core.Disconnected:
		return tui.StoppedStyle().Render(symbol)
	case core.ConnectionError:
		return tui.ErrorStyle().Render(symbol)
	default:
		return tui.ReconnectingStyle().Render(symbol)
	}
}

// RenderSessionBadge はセッション状態をカラーシンボルとして描画する（シンボルのみ）。
func RenderSessionBadge(status core.SessionStatus) string {
	symbol, ok := sessionSymbols[status]
	if !ok {
		return tui.MutedStyle().Render("?")
	}
	switch status {
	case core.Active:
		return tui.ActiveStyle().Render(symbol)
	case core.Stopped:
		return tui.StoppedStyle().Render(symbol)
	case core.SessionError:
		return tui.ErrorStyle().Render(symbol)
	default:
		return tui.ReconnectingStyle().Render(symbol)
	}
}
