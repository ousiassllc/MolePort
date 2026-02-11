package pages

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
	"github.com/ousiassllc/moleport/internal/tui/organisms"
)

// DashboardPage は3パネル + ステータスバーで構成されるレイアウト。
// Top: ForwardPanel (全ホストのアクティブフォワード)
// Middle: SetupPanel (ホスト選択 + ウィザード)
// Bottom: LogPanel (ログ出力) + StatusBar
type DashboardPage struct {
	forward   organisms.ForwardPanel
	setup     organisms.SetupPanel
	log       organisms.LogPanel
	statusBar organisms.StatusBar

	focusedPane tui.FocusPane
	width       int
	height      int
	version     string
}

// NewDashboardPage は新しい DashboardPage を生成する。
func NewDashboardPage(version string) DashboardPage {
	d := DashboardPage{
		forward:     organisms.NewForwardPanel(),
		setup:       organisms.NewSetupPanel(),
		log:         organisms.NewLogPanel(),
		statusBar:   organisms.NewStatusBar(),
		focusedPane: tui.PaneSetup,
		version:     version,
	}
	d.setup.SetFocused(true)
	return d
}

// Init は初期化コマンドを返す。
func (d DashboardPage) Init() tea.Cmd {
	return nil
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
		// Tab でフォーカス切替
		if msg.String() == "tab" {
			d.cycleFocus()
			return d, nil
		}

		// フォーカス中のパネルにキーを送る
		switch d.focusedPane {
		case tui.PaneForwards:
			var cmd tea.Cmd
			d.forward, cmd = d.forward.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case tui.PaneSetup:
			var cmd tea.Cmd
			d.setup, cmd = d.setup.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return d, tea.Batch(cmds...)

	// ブロードキャストメッセージ
	case tui.SSHEventMsg:
		d.handleSSHEvent(msg.Event)
	case tui.LogOutputMsg:
		d.log.AppendOutput(msg.Text)
	}

	// SetupPanel にメッセージを転送（テキスト入力の blink 等）
	var setupCmd tea.Cmd
	d.setup, setupCmd = d.setup.Update(msg)
	if setupCmd != nil {
		cmds = append(cmds, setupCmd)
	}

	return d, tea.Batch(cmds...)
}

// renderHeader は1行ヘッダーを描画する。
func (d DashboardPage) renderHeader() string {
	appName := tui.HeaderStyle.Render("  MolePort")
	version := tui.MutedStyle.Render(d.version)

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
	forwardView := d.forward.View()
	divider1 := atoms.RenderDivider(d.width)
	setupView := d.setup.View()
	divider2 := atoms.RenderDivider(d.width)
	logView := d.log.View()
	statusView := d.statusBar.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		forwardView,
		divider1,
		setupView,
		divider2,
		logView,
		statusView,
	)
}

// --- パネルへのアクセサ ---

// SetHosts はホスト一覧を設定する。
func (d *DashboardPage) SetHosts(hosts []core.SSHHost) {
	d.setup.SetHosts(hosts)
	d.updateStats()
}

// SetForwardSessions はフォワードセッション一覧を設定する。
func (d *DashboardPage) SetForwardSessions(sessions []core.ForwardSession) {
	d.forward.SetSessions(sessions)
	d.updateStats()
}

// UpdateHostState はホストの接続状態を更新する。
func (d *DashboardPage) UpdateHostState(hostName string, state core.ConnectionState) {
	d.setup.UpdateHostState(hostName, state)
	d.updateStats()
}

// AppendLog はログ出力を追加する。
func (d *DashboardPage) AppendLog(text string) {
	d.log.AppendOutput(text)
}

// FocusedPane は現在のフォーカスペインを返す。
func (d DashboardPage) FocusedPane() tui.FocusPane {
	return d.focusedPane
}

// IsInputActive はテキスト入力中かどうかを返す。
func (d DashboardPage) IsInputActive() bool {
	return d.focusedPane == tui.PaneSetup && d.setup.IsInputActive()
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
