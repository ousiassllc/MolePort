package organisms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// WizardStep はセットアップウィザードのステップを表す。
type WizardStep int

const (
	StepIdle       WizardStep = iota // ホスト一覧表示（デフォルト）
	StepSelectType                   // フォワード種別選択: Local/Remote/Dynamic
	StepLocalPort                    // ローカルポート入力
	StepRemoteHost                   // リモートホスト入力（Dynamic ではスキップ）
	StepRemotePort                   // リモートポート入力（Dynamic ではスキップ）
	StepRuleName                     // ルール名入力（任意）
	StepConfirm                      // 確認
)

// SetupPanel はホスト選択 + フォワード追加ウィザードを提供するパネル。
type SetupPanel struct {
	hosts      []core.SSHHost
	hostCursor int
	step       WizardStep
	typeCursor int
	typeOptions []string

	portInput textinput.Model
	hostInput textinput.Model
	nameInput textinput.Model

	// ウィザードで蓄積される値
	selectedHost string
	selectedType core.ForwardType
	localPort    string
	remoteHost   string
	remotePort   string
	ruleName     string

	focused bool
	width   int
	height  int
}

// NewSetupPanel は新しい SetupPanel を生成する。
func NewSetupPanel() SetupPanel {
	portIn := textinput.New()
	portIn.Placeholder = "8080"
	portIn.CharLimit = 5

	hostIn := textinput.New()
	hostIn.Placeholder = "localhost"
	hostIn.CharLimit = 256

	nameIn := textinput.New()
	nameIn.Placeholder = "任意のルール名"
	nameIn.CharLimit = 64

	return SetupPanel{
		typeOptions: []string{"Local (-L)", "Remote (-R)", "Dynamic (-D)"},
		portInput:   portIn,
		hostInput:   hostIn,
		nameInput:   nameIn,
	}
}

// SetFocused はフォーカス状態を設定する。
func (p *SetupPanel) SetFocused(focused bool) {
	p.focused = focused
}

// SetHosts はホスト一覧を設定する。
func (p *SetupPanel) SetHosts(hosts []core.SSHHost) {
	p.hosts = hosts
	if p.hostCursor >= len(hosts) {
		if len(hosts) > 0 {
			p.hostCursor = len(hosts) - 1
		} else {
			p.hostCursor = 0
		}
	}
}

// SetSize はパネルのサイズを設定する。
func (p *SetupPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Hosts は現在のホスト一覧を返す。
func (p SetupPanel) Hosts() []core.SSHHost {
	return p.hosts
}

// IsInputActive はテキスト入力中かどうかを返す。
func (p SetupPanel) IsInputActive() bool {
	switch p.step {
	case StepLocalPort, StepRemoteHost, StepRemotePort, StepRuleName:
		return true
	}
	return false
}

// UpdateHostState は指定ホストの状態を更新する。
func (p *SetupPanel) UpdateHostState(hostName string, state core.ConnectionState) {
	for i := range p.hosts {
		if p.hosts[i].Name == hostName {
			p.hosts[i].State = state
			break
		}
	}
}

// Update はキー入力を処理する。
func (p SetupPanel) Update(msg tea.Msg) (SetupPanel, tea.Cmd) {
	if !p.focused {
		return p, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// テキスト入力のステップではキー以外のメッセージも転送
		return p.updateTextInputs(msg)
	}

	keys := tui.DefaultKeyMap()

	// Esc でウィザードをキャンセルして StepIdle に戻る
	if key.Matches(keyMsg, keys.Escape) && p.step != StepIdle {
		p.resetWizard()
		return p, nil
	}

	switch p.step {
	case StepIdle:
		return p.updateIdle(keyMsg, keys)
	case StepSelectType:
		return p.updateSelectType(keyMsg, keys)
	case StepLocalPort, StepRemoteHost, StepRemotePort, StepRuleName:
		return p.updateTextInput(msg)
	case StepConfirm:
		return p.updateConfirm(keyMsg, keys)
	}

	return p, nil
}

func (p SetupPanel) updateTextInputs(msg tea.Msg) (SetupPanel, tea.Cmd) {
	switch p.step {
	case StepLocalPort, StepRemotePort:
		var cmd tea.Cmd
		p.portInput, cmd = p.portInput.Update(msg)
		return p, cmd
	case StepRemoteHost:
		var cmd tea.Cmd
		p.hostInput, cmd = p.hostInput.Update(msg)
		return p, cmd
	case StepRuleName:
		var cmd tea.Cmd
		p.nameInput, cmd = p.nameInput.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p SetupPanel) updateIdle(keyMsg tea.KeyMsg, keys tui.KeyMap) (SetupPanel, tea.Cmd) {
	prevCursor := p.hostCursor

	switch {
	case key.Matches(keyMsg, keys.Up):
		if p.hostCursor > 0 {
			p.hostCursor--
		}
	case key.Matches(keyMsg, keys.Down):
		if p.hostCursor < len(p.hosts)-1 {
			p.hostCursor++
		}
	case key.Matches(keyMsg, keys.Enter):
		if len(p.hosts) > 0 && p.hostCursor < len(p.hosts) {
			p.selectedHost = p.hosts[p.hostCursor].Name
			p.step = StepSelectType
			p.typeCursor = 0
		}
		return p, nil
	default:
		return p, nil
	}

	// カーソルが移動した場合に HostSelectedMsg を発行
	if prevCursor != p.hostCursor && len(p.hosts) > 0 {
		host := p.hosts[p.hostCursor]
		return p, func() tea.Msg {
			return tui.HostSelectedMsg{Host: host}
		}
	}

	return p, nil
}

func (p SetupPanel) updateSelectType(keyMsg tea.KeyMsg, keys tui.KeyMap) (SetupPanel, tea.Cmd) {
	switch {
	case key.Matches(keyMsg, keys.Up):
		if p.typeCursor > 0 {
			p.typeCursor--
		}
	case key.Matches(keyMsg, keys.Down):
		if p.typeCursor < len(p.typeOptions)-1 {
			p.typeCursor++
		}
	case key.Matches(keyMsg, keys.Enter):
		switch p.typeCursor {
		case 0:
			p.selectedType = core.Local
		case 1:
			p.selectedType = core.Remote
		case 2:
			p.selectedType = core.Dynamic
		}
		p.step = StepLocalPort
		p.portInput.Reset()
		p.portInput.Placeholder = "8080"
		p.portInput.Focus()
		return p, textinput.Blink
	}
	return p, nil
}

func (p SetupPanel) updateTextInput(msg tea.Msg) (SetupPanel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok && keyMsg.Type == tea.KeyEnter {
		var value string
		switch p.step {
		case StepLocalPort, StepRemotePort:
			value = p.portInput.Value()
		case StepRemoteHost:
			value = p.hostInput.Value()
		case StepRuleName:
			value = p.nameInput.Value()
		}
		return p.advanceFromTextStep(value)
	}

	return p.updateTextInputs(msg)
}

func (p SetupPanel) advanceFromTextStep(value string) (SetupPanel, tea.Cmd) {
	switch p.step {
	case StepLocalPort:
		if err := validatePortStr(value); err != nil {
			return p, nil // 無効な値は無視
		}
		p.localPort = value
		if p.selectedType == core.Dynamic {
			// Dynamic の場合は RemoteHost/RemotePort をスキップ
			p.remoteHost = ""
			p.remotePort = "0"
			p.step = StepRuleName
			p.nameInput.Reset()
			suggestion := fmt.Sprintf("%s-dynamic-%s", p.selectedHost, p.localPort)
			p.nameInput.Placeholder = suggestion
			p.nameInput.Focus()
			return p, textinput.Blink
		}
		p.step = StepRemoteHost
		p.hostInput.Reset()
		p.hostInput.Placeholder = "localhost"
		p.hostInput.Focus()
		return p, textinput.Blink

	case StepRemoteHost:
		if value == "" {
			value = "localhost"
		}
		p.remoteHost = value
		p.step = StepRemotePort
		p.portInput.Reset()
		p.portInput.Placeholder = "80"
		p.portInput.Focus()
		return p, textinput.Blink

	case StepRemotePort:
		if err := validatePortStr(value); err != nil {
			return p, nil
		}
		p.remotePort = value
		p.step = StepRuleName
		p.nameInput.Reset()
		typeStr := p.selectedType.String()
		suggestion := fmt.Sprintf("%s-%s-%s", p.selectedHost, typeStr, p.localPort)
		p.nameInput.Placeholder = suggestion
		p.nameInput.Focus()
		return p, textinput.Blink

	case StepRuleName:
		if value == "" {
			// プレースホルダーの値を使用
			value = p.nameInput.Placeholder
		}
		p.ruleName = value
		p.step = StepConfirm
		p.portInput.Blur()
		p.hostInput.Blur()
		p.nameInput.Blur()
		return p, nil
	}

	return p, nil
}

func (p SetupPanel) updateConfirm(keyMsg tea.KeyMsg, keys tui.KeyMap) (SetupPanel, tea.Cmd) {
	if key.Matches(keyMsg, keys.Enter) {
		localPort, _ := strconv.Atoi(p.localPort)
		remotePort, _ := strconv.Atoi(p.remotePort)

		msg := tui.ForwardAddRequestMsg{
			Host:        p.selectedHost,
			Type:        p.selectedType,
			LocalPort:   localPort,
			RemoteHost:  p.remoteHost,
			RemotePort:  remotePort,
			Name:        p.ruleName,
			AutoConnect: true,
		}

		p.resetWizard()

		return p, func() tea.Msg { return msg }
	}
	return p, nil
}

func (p *SetupPanel) resetWizard() {
	p.step = StepIdle
	p.typeCursor = 0
	p.selectedHost = ""
	p.localPort = ""
	p.remoteHost = ""
	p.remotePort = ""
	p.ruleName = ""
	p.portInput.Blur()
	p.hostInput.Blur()
	p.nameInput.Blur()
}

// View はパネルを描画する。
func (p SetupPanel) View() string {
	contentWidth := p.width
	if contentWidth < 10 {
		contentWidth = 10
	}

	var rows []string

	switch p.step {
	case StepIdle:
		rows = p.viewHostList(contentWidth)
	case StepSelectType:
		rows = p.viewSelectType()
	case StepLocalPort:
		rows = p.viewTextInput("Local port", &p.portInput)
	case StepRemoteHost:
		rows = p.viewTextInput("Remote host", &p.hostInput)
	case StepRemotePort:
		rows = p.viewTextInput("Remote port", &p.portInput)
	case StepRuleName:
		rows = p.viewTextInput("Rule name", &p.nameInput)
	case StepConfirm:
		rows = p.viewConfirm()
	}

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(contentWidth).Height(p.height).Render(content)
}

func (p SetupPanel) viewHostList(contentWidth int) []string {
	// タイトル
	countLabel := tui.MutedStyle.Render(fmt.Sprintf("(%d)", len(p.hosts)))
	var title string
	if p.focused {
		title = tui.FocusIndicator + " " + tui.SectionTitleStyle.Render("SSH Hosts") + " " + countLabel
	} else {
		title = "  " + tui.MutedStyle.Bold(true).Render("SSH Hosts") + " " + countLabel
	}

	var rows []string
	rows = append(rows, title)

	if len(p.hosts) == 0 {
		rows = append(rows, "  "+tui.MutedStyle.Render("ホストが見つかりません"))
	} else {
		maxRows := p.height - 1
		if maxRows < 1 {
			maxRows = 1
		}

		offset := 0
		if p.hostCursor >= maxRows {
			offset = p.hostCursor - maxRows + 1
		}

		end := offset + maxRows
		if end > len(p.hosts) {
			end = len(p.hosts)
		}

		for i := offset; i < end; i++ {
			row := molecules.HostRow{
				Host:     p.hosts[i],
				Selected: i == p.hostCursor,
				Width:    contentWidth,
			}
			prefix := "  "
			if i == p.hostCursor {
				prefix = tui.ActiveStyle.Render("> ")
			}
			rows = append(rows, prefix+row.View())
		}
	}

	return rows
}

func (p SetupPanel) wizardTitle() string {
	breadcrumb := fmt.Sprintf("New Forward %s %s",
		tui.MutedStyle.Render("→"),
		tui.TextStyle.Render(p.selectedHost),
	)
	if p.step > StepSelectType {
		breadcrumb += " " + tui.MutedStyle.Render("→") + " " + tui.TextStyle.Render(p.selectedType.String())
	}

	if p.focused {
		return tui.FocusIndicator + " " + tui.SectionTitleStyle.Render(breadcrumb)
	}
	return "  " + tui.MutedStyle.Bold(true).Render(breadcrumb)
}

func (p SetupPanel) viewSelectType() []string {
	var rows []string
	rows = append(rows, p.wizardTitle())
	rows = append(rows, "  "+tui.MutedStyle.Render("Select type:"))

	for i, opt := range p.typeOptions {
		cursor := "  "
		if i == p.typeCursor {
			cursor = tui.ActiveStyle.Render("> ")
			opt = tui.SelectedStyle.Render(opt)
		} else {
			opt = tui.TextStyle.Render(opt)
		}
		rows = append(rows, "  "+cursor+opt)
	}

	rows = append(rows, "")
	rows = append(rows, "  "+tui.MutedStyle.Render("[Enter] 選択  [Esc] キャンセル"))
	return rows
}

func (p SetupPanel) viewTextInput(label string, input *textinput.Model) []string {
	stepNum, totalSteps := p.stepProgress()

	var rows []string
	rows = append(rows, p.wizardTitle())
	rows = append(rows, "  "+tui.MutedStyle.Render(fmt.Sprintf("Step %d/%d", stepNum, totalSteps)))
	rows = append(rows, "  "+tui.TextStyle.Render(label+": ")+input.View())
	rows = append(rows, "")
	rows = append(rows, "  "+tui.MutedStyle.Render("[Enter] 次へ  [Esc] キャンセル"))
	return rows
}

func (p SetupPanel) viewConfirm() []string {
	var rows []string
	rows = append(rows, p.wizardTitle())
	rows = append(rows, "")

	if p.selectedType == core.Dynamic {
		rows = append(rows, "  "+tui.TextStyle.Render(fmt.Sprintf(":%s (SOCKS)", p.localPort)))
	} else {
		rows = append(rows, "  "+tui.TextStyle.Render(fmt.Sprintf(":%s %s %s:%s",
			p.localPort,
			tui.MutedStyle.Render("→"),
			p.remoteHost,
			p.remotePort,
		)))
	}

	rows = append(rows, "  "+tui.MutedStyle.Render("Name: ")+tui.TextStyle.Render(p.ruleName))
	rows = append(rows, "")
	rows = append(rows, "  "+tui.MutedStyle.Render("[Enter] 作成 & 接続  [Esc] キャンセル"))
	return rows
}

func (p SetupPanel) stepProgress() (current int, total int) {
	if p.selectedType == core.Dynamic {
		// StepSelectType(1) -> StepLocalPort(2) -> StepRuleName(3) -> StepConfirm(4)
		total = 4
		switch p.step {
		case StepSelectType:
			current = 1
		case StepLocalPort:
			current = 2
		case StepRuleName:
			current = 3
		case StepConfirm:
			current = 4
		}
	} else {
		// StepSelectType(1) -> StepLocalPort(2) -> StepRemoteHost(3) -> StepRemotePort(4) -> StepRuleName(5) -> StepConfirm(6)
		total = 6
		switch p.step {
		case StepSelectType:
			current = 1
		case StepLocalPort:
			current = 2
		case StepRemoteHost:
			current = 3
		case StepRemotePort:
			current = 4
		case StepRuleName:
			current = 5
		case StepConfirm:
			current = 6
		}
	}
	return
}

// validatePortStr はポート番号の文字列をバリデーションする。
func validatePortStr(s string) error {
	if s == "" {
		return fmt.Errorf("ポート番号を入力してください")
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("数値を入力してください")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("ポート番号は 1-65535 の範囲で指定してください")
	}
	return nil
}
