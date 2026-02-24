package pages

import (
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/organisms"
)

func (d *DashboardPage) cycleFocus() {
	switch d.focusedPane {
	case tui.PaneForwards:
		d.setFocus(tui.PaneSetup)
	case tui.PaneSetup:
		d.setFocus(tui.PaneForwards)
	}
}

func (d *DashboardPage) setFocus(pane tui.FocusPane) {
	d.focusedPane = pane
	d.forward.SetFocused(pane == tui.PaneForwards)
	d.setup.SetFocused(pane == tui.PaneSetup)
	d.statusBar.SetFocusedPane(pane)
}

func (d *DashboardPage) updateSizes() {
	if d.width <= 0 || d.height <= 0 {
		return
	}

	// レイアウト:
	//   Header:    1 line
	//   Forward:   ~40% of remaining
	//   Divider:   1 line
	//   Setup:     ~45% of remaining (残り全部)
	//   Divider:   1 line
	//   Log:       3 lines (固定)
	//   StatusBar: 1 line

	const logHeight = 3
	fixedLines := 1 + 1 + 1 + logHeight + 1 // header + divider1 + divider2 + log + statusbar
	remaining := d.height - fixedLines
	if remaining < 8 {
		remaining = 8
	}

	forwardHeight := remaining * 40 / 100
	if forwardHeight < 3 {
		forwardHeight = 3
	}

	setupHeight := remaining - forwardHeight
	if setupHeight < 5 {
		setupHeight = 5
	}

	d.forward.SetSize(d.width, forwardHeight)
	d.setup.SetSize(d.width, setupHeight)
	d.log.SetSize(d.width, logHeight)
	d.statusBar.SetWidth(d.width)
}

func (d *DashboardPage) handleSSHEvent(event core.SSHEvent) {
	switch event.Type {
	case core.SSHEventConnected:
		d.setup.UpdateHostState(event.HostName, core.Connected)
	case core.SSHEventDisconnected:
		d.setup.UpdateHostState(event.HostName, core.Disconnected)
	case core.SSHEventReconnecting:
		d.setup.UpdateHostState(event.HostName, core.Reconnecting)
	case core.SSHEventPendingAuth:
		d.setup.UpdateHostState(event.HostName, core.PendingAuth)
	case core.SSHEventError:
		d.setup.UpdateHostState(event.HostName, core.ConnectionError)
	}
	d.updateStats()
}

func (d *DashboardPage) updateStats() {
	hosts := d.setup.Hosts()
	sessions := d.forward.Sessions()

	var connected, activeForwards int
	for _, h := range hosts {
		if h.State == core.Connected {
			connected++
		}
	}
	for _, s := range sessions {
		if s.Status == core.Active {
			activeForwards++
		}
	}

	d.statusBar.SetStats(organisms.StatusBarStats{
		TotalHosts:     len(hosts),
		ConnectedHosts: connected,
		TotalForwards:  len(sessions),
		ActiveForwards: activeForwards,
	})
}
