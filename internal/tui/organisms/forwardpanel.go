package organisms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// ForwardPanel はポートフォワーディングセッション一覧を表示するパネル。
type ForwardPanel struct {
	sessions []core.ForwardSession
	hostName string
	cursor   int
	focused  bool
	width    int
	height   int
}

// NewForwardPanel は新しい ForwardPanel を生成する。
func NewForwardPanel() ForwardPanel {
	return ForwardPanel{}
}

// SetFocused はフォーカス状態を設定する。
func (p *ForwardPanel) SetFocused(focused bool) {
	p.focused = focused
}

// SetSessions はセッション一覧を設定する。
func (p *ForwardPanel) SetSessions(sessions []core.ForwardSession) {
	p.sessions = sessions
	if p.cursor >= len(sessions) {
		if len(sessions) > 0 {
			p.cursor = len(sessions) - 1
		} else {
			p.cursor = 0
		}
	}
}

// SetHostName は表示中のホスト名を設定する。
func (p *ForwardPanel) SetHostName(name string) {
	p.hostName = name
}

// SetSize はパネルのサイズを設定する。
func (p *ForwardPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SelectedSession は現在選択中のセッションを返す。
func (p ForwardPanel) SelectedSession() *core.ForwardSession {
	if len(p.sessions) == 0 || p.cursor >= len(p.sessions) {
		return nil
	}
	s := p.sessions[p.cursor]
	return &s
}

// Update はキー入力を処理する。
func (p ForwardPanel) Update(msg tea.Msg) (ForwardPanel, tea.Cmd) {
	if !p.focused {
		return p, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	keys := tui.DefaultKeyMap()

	switch {
	case key.Matches(keyMsg, keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
	case key.Matches(keyMsg, keys.Down):
		if p.cursor < len(p.sessions)-1 {
			p.cursor++
		}
	case key.Matches(keyMsg, keys.Enter):
		if s := p.selectedSession(); s != nil {
			return p, func() tea.Msg {
				return tui.ForwardToggleMsg{RuleName: s.Rule.Name}
			}
		}
	case key.Matches(keyMsg, keys.Disconnect):
		if s := p.selectedSession(); s != nil && s.Status == core.Active {
			return p, func() tea.Msg {
				return tui.ForwardToggleMsg{RuleName: s.Rule.Name}
			}
		}
	case key.Matches(keyMsg, keys.Delete):
		if s := p.selectedSession(); s != nil {
			return p, func() tea.Msg {
				return tui.ForwardDeleteRequestMsg{RuleName: s.Rule.Name}
			}
		}
	}

	return p, nil
}

func (p ForwardPanel) selectedSession() *core.ForwardSession {
	if len(p.sessions) == 0 || p.cursor >= len(p.sessions) {
		return nil
	}
	s := p.sessions[p.cursor]
	return &s
}

// View はパネルを描画する（ボーダーなし）。
func (p ForwardPanel) View() string {
	contentWidth := p.width
	if contentWidth < 10 {
		contentWidth = 10
	}

	// セクションタイトル
	titleText := "Forwards"
	if p.hostName != "" {
		titleText = "Forwards " + tui.MutedStyle.Render("["+p.hostName+"]")
	}
	var title string
	if p.focused {
		title = tui.FocusIndicator + " " + tui.SectionTitleStyle.Render(titleText)
	} else {
		title = "  " + tui.MutedStyle.Bold(true).Render(titleText)
	}

	var rows []string
	rows = append(rows, title)

	if len(p.sessions) == 0 {
		rows = append(rows, "  "+tui.MutedStyle.Render("フォワーディングルールがありません"))
	} else {
		maxRows := p.height - 1
		if maxRows < 1 {
			maxRows = 1
		}

		offset := 0
		if p.cursor >= maxRows {
			offset = p.cursor - maxRows + 1
		}

		end := offset + maxRows
		if end > len(p.sessions) {
			end = len(p.sessions)
		}

		for i := offset; i < end; i++ {
			row := molecules.ForwardRow{
				Session:  p.sessions[i],
				Selected: i == p.cursor,
				Width:    contentWidth,
			}
			rows = append(rows, "  "+row.View())
		}
	}

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(contentWidth).Height(p.height).Render(content)
}

// Sessions は現在のセッション一覧を返す。
func (p ForwardPanel) Sessions() []core.ForwardSession {
	return p.sessions
}
