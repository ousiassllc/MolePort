package setuppanel

import (
	"errors"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
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

// Panel はホスト選択 + フォワード追加ウィザードを提供するパネル。
type Panel struct {
	hosts       []core.SSHHost
	hostCursor  int
	step        WizardStep
	typeCursor  int
	typeOptions []string
	keys        tui.KeyMap

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

// New は新しい Panel を生成する。
func New() Panel {
	portIn := textinput.New()
	portIn.Placeholder = "8080"
	portIn.CharLimit = 5

	hostIn := textinput.New()
	hostIn.Placeholder = "localhost"
	hostIn.CharLimit = 256

	nameIn := textinput.New()
	nameIn.Placeholder = i18n.T("tui.setup_panel.rule_name_placeholder")
	nameIn.CharLimit = 64

	return Panel{
		typeOptions: []string{"Local (-L)", "Remote (-R)", "Dynamic (-D)"},
		keys:        tui.DefaultKeyMap(),
		portInput:   portIn,
		hostInput:   hostIn,
		nameInput:   nameIn,
	}
}

// SetFocused はフォーカス状態を設定する。
func (p *Panel) SetFocused(focused bool) {
	p.focused = focused
}

// SetHosts はホスト一覧を設定する。
func (p *Panel) SetHosts(hosts []core.SSHHost) {
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
func (p *Panel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Hosts は現在のホスト一覧を返す。
func (p Panel) Hosts() []core.SSHHost {
	return p.hosts
}

// IsInputActive はテキスト入力中かどうかを返す。
func (p Panel) IsInputActive() bool {
	switch p.step {
	case StepLocalPort, StepRemoteHost, StepRemotePort, StepRuleName:
		return true
	}
	return false
}

// UpdateHostState は指定ホストの状態を更新する。
func (p *Panel) UpdateHostState(hostName string, state core.ConnectionState) {
	for i := range p.hosts {
		if p.hosts[i].Name == hostName {
			p.hosts[i].State = state
			break
		}
	}
}

// Update はキー入力を処理する。
func (p Panel) Update(msg tea.Msg) (Panel, tea.Cmd) {
	if !p.focused {
		return p, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// テキスト入力のステップではキー以外のメッセージも転送
		return p.updateTextInputs(msg)
	}

	// Esc でウィザードをキャンセルして StepIdle に戻る
	if key.Matches(keyMsg, p.keys.Escape) && p.step != StepIdle {
		p.resetWizard()
		return p, nil
	}

	switch p.step {
	case StepIdle:
		return p.updateIdle(keyMsg, p.keys)
	case StepSelectType:
		return p.updateSelectType(keyMsg, p.keys)
	case StepLocalPort, StepRemoteHost, StepRemotePort, StepRuleName:
		return p.updateTextInput(msg)
	case StepConfirm:
		return p.updateConfirm(keyMsg, p.keys)
	}

	return p, nil
}

func (p *Panel) resetWizard() {
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

// wizardSteps はフォワード種別ごとのウィザードステップ順序を定義する。
var wizardSteps = map[bool][]WizardStep{
	true:  {StepSelectType, StepLocalPort, StepRuleName, StepConfirm},                                 // Dynamic
	false: {StepSelectType, StepLocalPort, StepRemoteHost, StepRemotePort, StepRuleName, StepConfirm}, // Local/Remote
}

func (p Panel) stepProgress() (current int, total int) {
	steps := wizardSteps[p.selectedType == core.Dynamic]
	total = len(steps)
	for i, s := range steps {
		if s == p.step {
			current = i + 1
			break
		}
	}
	return
}

// validatePortStr はポート番号の文字列をバリデーションする。
func validatePortStr(s string) error {
	if s == "" {
		return errors.New(i18n.T("tui.setup_panel.port_required"))
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return errors.New(i18n.T("tui.setup_panel.port_not_number"))
	}
	if port < 1 || port > 65535 {
		return errors.New(i18n.T("tui.setup_panel.port_out_of_range"))
	}
	return nil
}
