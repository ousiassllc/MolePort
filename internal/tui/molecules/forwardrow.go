package molecules

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
)

// ForwardRow はポートフォワーディングセッション1行分の表示を担う。
type ForwardRow struct {
	Session  core.ForwardSession
	HostName string
	Selected bool
	Width    int
}

// forwardTypeLabel は転送種別の短縮表記を返す。
func forwardTypeLabel(t core.ForwardType) string {
	switch t {
	case core.Local:
		return "L"
	case core.Remote:
		return "R"
	case core.Dynamic:
		return "D"
	default:
		return "?"
	}
}

// View は ForwardRow を描画する。
// 形式: "● [host] L :8080 ──▸ remote:80     2h15m  ↑1.2MB ↓340KB"
func (r ForwardRow) View() string {
	badge := atoms.RenderSessionBadge(r.Session.Status)

	hostLabel := ""
	if r.HostName != "" {
		hostLabel = tui.MutedStyle.Render("["+r.HostName+"]") + " "
	}

	typeLabel := tui.ActiveStyle.Render(forwardTypeLabel(r.Session.Rule.Type))

	localPort := atoms.RenderPortLabel(r.Session.Rule.LocalPort)

	arrow := tui.DividerStyle.Render("──▸")

	var route string
	if r.Session.Rule.Type == core.Dynamic {
		route = tui.MutedStyle.Render("(SOCKS)")
	} else {
		route = tui.MutedStyle.Render(
			fmt.Sprintf("%s:%d", r.Session.Rule.RemoteHost, r.Session.Rule.RemotePort),
		)
	}

	var uptime string
	if r.Session.Status == core.Active && !r.Session.ConnectedAt.IsZero() {
		uptime = atoms.RenderDuration(time.Since(r.Session.ConnectedAt))
	}

	traffic := atoms.RenderTraffic(r.Session.BytesSent, r.Session.BytesReceived)

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		badge, " ", hostLabel, typeLabel, " ", localPort, " ", arrow, " ", route,
	)
	if uptime != "" {
		row = lipgloss.JoinHorizontal(lipgloss.Top, row, "  ", uptime)
	}
	row = lipgloss.JoinHorizontal(lipgloss.Top, row, "  ", traffic)

	return row
}
