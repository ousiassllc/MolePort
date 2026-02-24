package organisms

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

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
