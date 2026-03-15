package organisms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// ForwardPanel はポートフォワーディングセッション一覧を表示するパネル。
// 全ホストのフォワードを一括表示する。
type ForwardPanel struct {
	sessions []core.ForwardSession
	cursor   int
	keys     tui.KeyMap
	focused  bool
	width    int
	height   int
}

// NewForwardPanel は新しい ForwardPanel を生成する。
func NewForwardPanel() ForwardPanel {
	return ForwardPanel{
		keys: tui.DefaultKeyMap(),
	}
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

// SetSize はパネルのサイズを設定する。
func (p *ForwardPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
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

	switch {
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.sessions)-1 {
			p.cursor++
		}
	case key.Matches(keyMsg, p.keys.Enter):
		if s := p.selectedSession(); s != nil {
			return p, func() tea.Msg {
				return tui.ForwardToggleMsg{RuleName: s.Rule.Name}
			}
		}
	case key.Matches(keyMsg, p.keys.Disconnect):
		if s := p.selectedSession(); s != nil && s.Status == core.Active {
			return p, func() tea.Msg {
				return tui.ForwardToggleMsg{RuleName: s.Rule.Name}
			}
		}
	case key.Matches(keyMsg, p.keys.Delete):
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

// View はパネルを描画する。
func (p ForwardPanel) View() string {
	innerWidth, innerHeight := PanelInnerSize(p.width, p.height)

	title := i18n.T("tui.forward.title", map[string]any{"Count": len(p.sessions)})

	var rows []string

	if len(p.sessions) == 0 {
		rows = append(rows, tui.MutedStyle().Render(i18n.T("tui.forward.empty")))
	} else {
		maxRows := innerHeight
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
				HostName: p.sessions[i].Rule.Host,
				Selected: i == p.cursor,
				Width:    innerWidth,
			}
			var prefix string
			if i == p.cursor {
				prefix = tui.ActiveStyle().Render("> ")
			}
			rows = append(rows, prefix+row.View())
		}
	}

	border := tui.UnfocusedBorder()
	if p.focused {
		border = tui.FocusedBorder()
	}

	content := strings.Join(rows, "\n")
	return tui.RenderWithBorderTitle(border, innerWidth, innerHeight, title, content)
}

// Sessions は現在のセッション一覧を返す。
func (p ForwardPanel) Sessions() []core.ForwardSession {
	return p.sessions
}
