package pages

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
	"github.com/ousiassllc/moleport/internal/tui/organisms"
)

// DashboardPage は4つのオーガニズムを組み合わせたレイアウト。
type DashboardPage struct {
	hostList  organisms.HostListPanel
	forward   organisms.ForwardPanel
	command   organisms.CommandPanel
	statusBar organisms.StatusBar

	focusedPane tui.FocusPane
	width       int
	height      int
}

// NewDashboardPage は新しい DashboardPage を生成する。
func NewDashboardPage() DashboardPage {
	d := DashboardPage{
		hostList:    organisms.NewHostListPanel(),
		forward:     organisms.NewForwardPanel(),
		command:     organisms.NewCommandPanel(),
		statusBar:   organisms.NewStatusBar(),
		focusedPane: tui.PaneHostList,
	}
	d.hostList.SetFocused(true)
	return d
}

// Init は初期化コマンドを返す。
func (d DashboardPage) Init() tea.Cmd {
	return d.command.FocusInput()
}

// Update はメッセージを処理する。
func (d DashboardPage) Update(msg tea.Msg) (DashboardPage, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		d.updateSizes()
		return d, nil

	case tea.KeyMsg:
		// グローバルキー処理
		switch msg.String() {
		case "tab":
			d.cycleFocus()
			return d, nil
		case "/":
			if d.focusedPane != tui.PaneCommand {
				d.setFocus(tui.PaneCommand)
			}
			return d, nil
		case "esc":
			if d.focusedPane == tui.PaneCommand && !d.command.IsInFlow() {
				d.setFocus(tui.PaneHostList)
				return d, nil
			}
		}

		// フォーカス中のパネルにキーを送る
		switch d.focusedPane {
		case tui.PaneHostList:
			var cmd tea.Cmd
			d.hostList, cmd = d.hostList.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case tui.PaneForward:
			var cmd tea.Cmd
			d.forward, cmd = d.forward.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case tui.PaneCommand:
			var cmd tea.Cmd
			d.command, cmd = d.command.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return d, tea.Batch(cmds...)

	// ブロードキャストメッセージ: 全パネルに送る
	case tui.SSHEventMsg:
		d.handleSSHEvent(msg.Event)
	case tui.ForwardUpdatedMsg:
		// ForwardPanel は親（app.go）が更新する
	case tui.HostsLoadedMsg, tui.HostsReloadedMsg:
		// ホスト一覧は親が SetHosts で更新する
	case tui.CommandOutputMsg:
		d.command.AppendOutput(msg.Text)
	}

	// PromptSubmitMsg を CommandPanel に渡す
	if _, ok := msg.(tui.CommandOutputMsg); !ok {
		var cmd tea.Cmd
		d.command, cmd = d.command.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return d, tea.Batch(cmds...)
}

// renderHeader は1行ヘッダーを描画する。
func (d DashboardPage) renderHeader() string {
	appName := tui.HeaderStyle.Render("  MolePort")
	version := tui.MutedStyle.Render("v0.1.0")

	gap := d.width - lipgloss.Width(appName) - lipgloss.Width(version) - 1
	if gap < 1 {
		return appName
	}

	padding := lipgloss.NewStyle().Width(gap).Render("")
	return appName + padding + version
}

// View はダッシュボードを描画する。
func (d DashboardPage) View() string {
	if d.width == 0 || d.height == 0 {
		return "Loading..."
	}

	header := d.renderHeader()
	hostView := d.hostList.View()
	divider1 := atoms.RenderDivider(d.width)
	forwardView := d.forward.View()
	divider2 := atoms.RenderDivider(d.width)
	commandView := d.command.View()
	statusView := d.statusBar.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		hostView,
		divider1,
		forwardView,
		divider2,
		commandView,
		statusView,
	)
}

// --- パネルへのアクセサ ---

// SetHosts はホスト一覧を設定する。
func (d *DashboardPage) SetHosts(hosts []core.SSHHost) {
	d.hostList.SetHosts(hosts)
	d.command.SetHostNames(d.hostList.HostNames())
	d.updateStats()
}

// SetForwardSessions はフォワードセッション一覧を設定する。
func (d *DashboardPage) SetForwardSessions(sessions []core.ForwardSession) {
	d.forward.SetSessions(sessions)
	d.command.SetSessions(sessions)
	d.updateStats()
}

// SetForwardRules はフォワードルール一覧を設定する。
func (d *DashboardPage) SetForwardRules(rules []core.ForwardRule) {
	d.command.SetRules(rules)
}

// SetSelectedHostName は選択中のホスト名を Forward パネルに設定する。
func (d *DashboardPage) SetSelectedHostName(name string) {
	d.forward.SetHostName(name)
}

// SelectedHost は HostListPanel で選択中のホストを返す。
func (d DashboardPage) SelectedHost() *core.SSHHost {
	return d.hostList.SelectedHost()
}

// UpdateHostState はホストの接続状態を更新する。
func (d *DashboardPage) UpdateHostState(hostName string, state core.ConnectionState) {
	d.hostList.UpdateHostState(hostName, state)
	d.updateStats()
}

// AppendCommandOutput はコマンド出力を追加する。
func (d *DashboardPage) AppendCommandOutput(text string) {
	d.command.AppendOutput(text)
}

// FocusedPane は現在のフォーカスペインを返す。
func (d DashboardPage) FocusedPane() tui.FocusPane {
	return d.focusedPane
}

// HostList は HostListPanel を返す。
func (d DashboardPage) HostList() organisms.HostListPanel {
	return d.hostList
}

// SetSize はサイズを設定する。
func (d *DashboardPage) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.updateSizes()
}

// --- 内部メソッド ---

func (d *DashboardPage) cycleFocus() {
	switch d.focusedPane {
	case tui.PaneHostList:
		d.setFocus(tui.PaneForward)
	case tui.PaneForward:
		d.setFocus(tui.PaneCommand)
	case tui.PaneCommand:
		d.setFocus(tui.PaneHostList)
	}
}

func (d *DashboardPage) setFocus(pane tui.FocusPane) {
	d.focusedPane = pane
	d.hostList.SetFocused(pane == tui.PaneHostList)
	d.forward.SetFocused(pane == tui.PaneForward)
	d.command.SetFocused(pane == tui.PaneCommand)
	d.statusBar.SetFocusedPane(pane)
}

func (d *DashboardPage) updateSizes() {
	if d.width <= 0 || d.height <= 0 {
		return
	}

	// レイアウト:
	//   Header:    1 line
	//   HostList:  ~30% of remaining
	//   Divider:   1 line
	//   Forward:   ~40% of remaining
	//   Divider:   1 line
	//   Command:   ~30% of remaining (min 4 lines)
	//   StatusBar: 1 line

	fixedLines := 1 + 1 + 1 + 1 // header + divider1 + divider2 + statusbar
	remaining := d.height - fixedLines
	if remaining < 6 {
		remaining = 6
	}

	hostHeight := remaining * 30 / 100
	if hostHeight < 2 {
		hostHeight = 2
	}

	commandHeight := remaining * 30 / 100
	if commandHeight < 4 {
		commandHeight = 4
	}

	forwardHeight := remaining - hostHeight - commandHeight
	if forwardHeight < 2 {
		forwardHeight = 2
	}

	d.hostList.SetSize(d.width, hostHeight)
	d.forward.SetSize(d.width, forwardHeight)
	d.command.SetSize(d.width, commandHeight)
	d.statusBar.SetWidth(d.width)
}

func (d *DashboardPage) handleSSHEvent(event core.SSHEvent) {
	switch event.Type {
	case core.SSHEventConnected:
		d.hostList.UpdateHostState(event.HostName, core.Connected)
	case core.SSHEventDisconnected:
		d.hostList.UpdateHostState(event.HostName, core.Disconnected)
	case core.SSHEventReconnecting:
		d.hostList.UpdateHostState(event.HostName, core.Reconnecting)
	case core.SSHEventError:
		d.hostList.UpdateHostState(event.HostName, core.ConnectionError)
	}
	d.updateStats()
}

func (d *DashboardPage) updateStats() {
	hosts := d.hostList.Hosts()
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
