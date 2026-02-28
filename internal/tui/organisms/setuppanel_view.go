package organisms

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// View はパネルを描画する。
func (p SetupPanel) View() string {
	// innerWidth = p.width - 4 (2 border + 2 padding)
	innerWidth := p.width - 4
	if innerWidth < 10 {
		innerWidth = 10
	}
	// innerHeight = p.height - 2 (top + bottom border)
	innerHeight := p.height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	var title string
	var rows []string

	switch p.step {
	case StepIdle:
		title = fmt.Sprintf("SSH Hosts (%d)", len(p.hosts))
		rows = p.viewHostList(innerWidth, innerHeight)
	case StepSelectType:
		title = p.wizardTitleText()
		rows = p.viewSelectType()
	case StepLocalPort:
		title = p.wizardTitleText()
		rows = p.viewTextInput("Local port", &p.portInput)
	case StepRemoteHost:
		title = p.wizardTitleText()
		rows = p.viewTextInput("Remote host", &p.hostInput)
	case StepRemotePort:
		title = p.wizardTitleText()
		rows = p.viewTextInput("Remote port", &p.portInput)
	case StepRuleName:
		title = p.wizardTitleText()
		rows = p.viewTextInput("Rule name", &p.nameInput)
	case StepConfirm:
		title = p.wizardTitleText()
		rows = p.viewConfirm()
	}

	border := tui.UnfocusedBorder
	if p.focused {
		border = tui.FocusedBorder
	}

	content := strings.Join(rows, "\n")
	return tui.RenderWithBorderTitle(border, innerWidth, innerHeight, title, content)
}

func (p SetupPanel) viewHostList(innerWidth, innerHeight int) []string {
	var rows []string

	if len(p.hosts) == 0 {
		rows = append(rows, tui.MutedStyle.Render("ホストが見つかりません"))
	} else {
		maxRows := innerHeight
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
				Width:    innerWidth,
			}
			var prefix string
			if i == p.hostCursor {
				prefix = tui.ActiveStyle.Render("> ")
			}
			rows = append(rows, prefix+row.View())
		}
	}

	return rows
}

func (p SetupPanel) wizardTitleText() string {
	title := fmt.Sprintf("New Forward > %s", p.selectedHost)
	if p.step > StepSelectType {
		title += " > " + p.selectedType.String()
	}
	return title
}

func (p SetupPanel) viewSelectType() []string {
	var rows []string
	rows = append(rows, tui.MutedStyle.Render("Select type:"))

	for i, opt := range p.typeOptions {
		if i == p.typeCursor {
			rows = append(rows, tui.ActiveStyle.Render("> ")+tui.SelectedStyle.Render(opt))
		} else {
			rows = append(rows, "  "+tui.TextStyle.Render(opt))
		}
	}

	rows = append(rows, "")
	rows = append(rows, tui.MutedStyle.Render("[Enter] 選択  [Esc] キャンセル"))
	return rows
}

func (p SetupPanel) viewTextInput(label string, input *textinput.Model) []string {
	stepNum, totalSteps := p.stepProgress()

	var rows []string
	rows = append(rows, tui.MutedStyle.Render(fmt.Sprintf("Step %d/%d", stepNum, totalSteps)))
	rows = append(rows, tui.TextStyle.Render(label+": ")+input.View())
	rows = append(rows, "")
	rows = append(rows, tui.MutedStyle.Render("[Enter] 次へ  [Esc] キャンセル"))
	return rows
}

func (p SetupPanel) viewConfirm() []string {
	var rows []string
	rows = append(rows, "")

	if p.selectedType == core.Dynamic {
		rows = append(rows, tui.TextStyle.Render(fmt.Sprintf(":%s (SOCKS)", p.localPort)))
	} else {
		rows = append(rows, tui.TextStyle.Render(fmt.Sprintf(":%s %s %s:%s",
			p.localPort,
			tui.MutedStyle.Render("→"),
			p.remoteHost,
			p.remotePort,
		)))
	}

	rows = append(rows, tui.MutedStyle.Render("Name: ")+tui.TextStyle.Render(p.ruleName))
	rows = append(rows, "")
	rows = append(rows, tui.MutedStyle.Render("[Enter] 作成 & 接続  [Esc] キャンセル"))
	return rows
}
