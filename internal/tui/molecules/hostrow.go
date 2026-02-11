package molecules

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
)

// HostRow は SSH ホスト1行分の表示を担う。
type HostRow struct {
	Host     core.SSHHost
	Selected bool
	Width    int
}

// View は HostRow を描画する。
// 形式: "● hostname              user@addr:22     2 fwd"
func (r HostRow) View() string {
	badge := atoms.RenderConnectionBadge(r.Host.State)

	nameStyle := tui.TextStyle
	if r.Selected {
		nameStyle = nameStyle.Bold(true).Foreground(tui.Accent)
	}
	name := nameStyle.Render(r.Host.Name)

	addr := tui.MutedStyle.Render(
		fmt.Sprintf("%s@%s:%d", r.Host.User, r.Host.HostName, r.Host.Port),
	)

	var forwards string
	if r.Host.ActiveForwardCount > 0 {
		forwards = tui.ActiveStyle.Render(
			fmt.Sprintf("%d fwd", r.Host.ActiveForwardCount),
		)
	} else {
		forwards = tui.MutedStyle.Render("0 fwd")
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		badge, " ", name, "  ", addr, "  ", forwards,
	)

	if r.Selected {
		rowWidth := r.Width
		if rowWidth <= 0 {
			rowWidth = lipgloss.Width(row)
		}
		return lipgloss.NewStyle().
			Background(tui.BgHighlight).
			Width(rowWidth).
			Render(row)
	}
	return row
}
