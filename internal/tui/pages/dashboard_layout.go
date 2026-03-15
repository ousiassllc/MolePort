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

	const (
		headerHeight         = 1
		logHeight            = 5 // 3 content + 2 border
		statusBarHeight      = 1
		forwardHeightPercent = 40 // remaining の何%をフォワードパネルに割り当てるか
		minForwardHeight     = 3
		minSetupHeight       = 5
		minTotalHeight       = 8
	)

	fixedLines := headerHeight + logHeight + statusBarHeight
	remaining := d.height - fixedLines
	if remaining < minTotalHeight {
		remaining = minTotalHeight
	}

	forwardHeight := remaining * forwardHeightPercent / 100
	if forwardHeight < minForwardHeight {
		forwardHeight = minForwardHeight
	}

	setupHeight := remaining - forwardHeight
	if setupHeight < minSetupHeight {
		setupHeight = minSetupHeight
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
