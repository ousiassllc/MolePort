package setuppanel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// panelInnerSize はパネルの外枠サイズから内部描画領域のサイズを計算する。
// ボーダー分（幅: 左右各2, 高さ: 上下各1）を差し引き、最小値でクランプする。
func panelInnerSize(width, height int) (innerWidth, innerHeight int) {
	innerWidth = max(width-4, 10)
	innerHeight = max(height-2, 1)
	return
}

// View はパネルを描画する。
func (p Panel) View() string {
	innerWidth, innerHeight := panelInnerSize(p.width, p.height)

	var title string
	var rows []string

	switch p.step {
	case StepIdle:
		title = i18n.T("tui.setup_panel.title", map[string]any{"Count": len(p.hosts)})
		rows = p.viewHostList(innerWidth, innerHeight)
	case StepSelectType:
		title = p.wizardTitleText()
		rows = p.viewSelectType()
	case StepLocalPort:
		title = p.wizardTitleText()
		rows = p.viewTextInput(i18n.T("tui.setup_panel.label_local_port"), &p.portInput)
	case StepRemoteHost:
		title = p.wizardTitleText()
		rows = p.viewTextInput(i18n.T("tui.setup_panel.label_remote_host"), &p.hostInput)
	case StepRemotePort:
		title = p.wizardTitleText()
		rows = p.viewTextInput(i18n.T("tui.setup_panel.label_remote_port"), &p.portInput)
	case StepRuleName:
		title = p.wizardTitleText()
		rows = p.viewTextInput(i18n.T("tui.setup_panel.label_rule_name"), &p.nameInput)
	case StepConfirm:
		title = p.wizardTitleText()
		rows = p.viewConfirm()
	}

	border := tui.UnfocusedBorder()
	if p.focused {
		border = tui.FocusedBorder()
	}

	content := strings.Join(rows, "\n")
	return tui.RenderWithBorderTitle(border, innerWidth, innerHeight, title, content)
}

func (p Panel) viewHostList(innerWidth, innerHeight int) []string {
	var rows []string

	if len(p.hosts) == 0 {
		rows = append(rows, tui.MutedStyle().Render(i18n.T("tui.setup_panel.no_hosts")))
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
				prefix = tui.ActiveStyle().Render("> ")
			}
			rows = append(rows, prefix+row.View())
		}
	}

	return rows
}

func (p Panel) wizardTitleText() string {
	title := i18n.T("tui.setup_panel.wizard_title") + " > " + p.selectedHost
	if p.step > StepSelectType {
		title += " > " + p.selectedType.String()
	}
	return title
}

func (p Panel) viewSelectType() []string {
	var rows []string
	rows = append(rows, tui.MutedStyle().Render(i18n.T("tui.setup_panel.select_type")))

	for i, opt := range p.typeOptions {
		if i == p.typeCursor {
			rows = append(rows, tui.ActiveStyle().Render("> ")+tui.SelectedStyle().Render(opt))
		} else {
			rows = append(rows, "  "+tui.TextStyle().Render(opt))
		}
	}

	rows = append(rows, "")
	rows = append(rows, tui.MutedStyle().Render(i18n.T("tui.setup_panel.enter_select")))
	return rows
}

func (p Panel) viewTextInput(label string, input *textinput.Model) []string {
	stepNum, totalSteps := p.stepProgress()

	var rows []string
	rows = append(rows, tui.MutedStyle().Render(i18n.T("tui.setup_panel.step_progress", map[string]any{"Current": stepNum, "Total": totalSteps})))
	rows = append(rows, tui.TextStyle().Render(label+": ")+input.View())
	rows = append(rows, "")
	rows = append(rows, tui.MutedStyle().Render(i18n.T("tui.setup_panel.enter_next")))
	return rows
}

func (p Panel) viewConfirm() []string {
	var rows []string
	rows = append(rows, "")

	if p.selectedType == core.Dynamic {
		rows = append(rows, tui.TextStyle().Render(fmt.Sprintf(":%s (SOCKS)", p.localPort)))
	} else {
		rows = append(rows, tui.TextStyle().Render(fmt.Sprintf(":%s %s %s:%s",
			p.localPort,
			tui.MutedStyle().Render("→"),
			p.remoteHost,
			p.remotePort,
		)))
	}

	rows = append(rows, tui.MutedStyle().Render("Name: ")+tui.TextStyle().Render(p.ruleName))
	rows = append(rows, "")
	rows = append(rows, tui.MutedStyle().Render(i18n.T("tui.setup_panel.enter_create")))
	return rows
}
