package organisms

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
)

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
