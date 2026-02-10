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
	Selected bool
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
// 形式: "L  :8080 → remote:80  ● Active  2h 15m  ↑1.2MB ↓340KB"
func (r ForwardRow) View() string {
	typeLabel := tui.TitleStyle.Render(forwardTypeLabel(r.Session.Rule.Type))

	localPort := atoms.RenderPortLabel(r.Session.Rule.LocalPort)

	var route string
	if r.Session.Rule.Type == core.Dynamic {
		route = tui.MutedStyle.Render("(SOCKS)")
	} else {
		route = tui.MutedStyle.Render(
			fmt.Sprintf("→ %s:%d", r.Session.Rule.RemoteHost, r.Session.Rule.RemotePort),
		)
	}

	badge := atoms.RenderSessionBadge(r.Session.Status)

	var uptime string
	if r.Session.Status == core.Active && !r.Session.ConnectedAt.IsZero() {
		uptime = atoms.RenderDuration(time.Since(r.Session.ConnectedAt))
	}

	traffic := fmt.Sprintf("↑%s ↓%s",
		atoms.RenderDataSize(r.Session.BytesSent),
		atoms.RenderDataSize(r.Session.BytesReceived),
	)

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		typeLabel, "  ", localPort, " ", route, "  ", badge,
	)
	if uptime != "" {
		row = lipgloss.JoinHorizontal(lipgloss.Top, row, "  ", uptime)
	}
	row = lipgloss.JoinHorizontal(lipgloss.Top, row, "  ", traffic)

	if r.Selected {
		return tui.SelectedStyle.Render(row)
	}
	return row
}
