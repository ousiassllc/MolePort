package atoms

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

// 接続状態に対応するシンボルとスタイルの定義
var connectionBadges = map[core.ConnectionState]struct {
	symbol string
	style  lipgloss.Style
}{
	core.Connected:       {"●", tui.ActiveStyle},
	core.Disconnected:    {"○", tui.StoppedStyle},
	core.ConnectionError: {"✗", tui.ErrorStyle},
	core.Reconnecting:    {"↻", tui.ReconnectingStyle},
	core.Connecting:      {"◐", tui.ReconnectingStyle},
}

// セッション状態に対応するシンボルとスタイルの定義
var sessionBadges = map[core.SessionStatus]struct {
	symbol string
	style  lipgloss.Style
}{
	core.Active:              {"●", tui.ActiveStyle},
	core.Stopped:             {"○", tui.StoppedStyle},
	core.SessionError:        {"✗", tui.ErrorStyle},
	core.SessionReconnecting: {"↻", tui.ReconnectingStyle},
	core.Starting:            {"◐", tui.ReconnectingStyle},
}

// RenderConnectionBadge は SSH 接続状態をカラーシンボル付きテキストとして描画する。
func RenderConnectionBadge(state core.ConnectionState) string {
	if badge, ok := connectionBadges[state]; ok {
		return badge.style.Render(badge.symbol + " " + state.String())
	}
	return tui.MutedStyle.Render("? " + state.String())
}

// RenderSessionBadge はセッション状態をカラーシンボル付きテキストとして描画する。
func RenderSessionBadge(status core.SessionStatus) string {
	if badge, ok := sessionBadges[status]; ok {
		return badge.style.Render(badge.symbol + " " + status.String())
	}
	return tui.MutedStyle.Render("? " + status.String())
}
