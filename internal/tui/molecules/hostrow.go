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
}

// View は HostRow を描画する。
// 形式: "● Connected  my-server  user@hostname:22  [3 forwards]"
func (r HostRow) View() string {
	badge := atoms.RenderConnectionBadge(r.Host.State)

	name := tui.TitleStyle.Render(r.Host.Name)

	addr := tui.MutedStyle.Render(
		fmt.Sprintf("%s@%s:%d", r.Host.User, r.Host.HostName, r.Host.Port),
	)

	var forwards string
	if r.Host.ActiveForwardCount > 0 {
		forwards = tui.ActiveStyle.Render(
			fmt.Sprintf("[%d forwards]", r.Host.ActiveForwardCount),
		)
	} else {
		forwards = tui.MutedStyle.Render("[0 forwards]")
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		badge, "  ", name, "  ", addr, "  ", forwards,
	)

	if r.Selected {
		return tui.SelectedStyle.Render(row)
	}
	return row
}
