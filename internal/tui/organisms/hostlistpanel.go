package organisms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/atoms"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

// HostListPanel は SSH ホスト一覧をカーソル付きで表示するパネル。
type HostListPanel struct {
	hosts   []core.SSHHost
	cursor  int
	focused bool
	width   int
	height  int
}

// NewHostListPanel は新しい HostListPanel を生成する。
func NewHostListPanel() HostListPanel {
	return HostListPanel{}
}

// SetFocused はフォーカス状態を設定する。
func (p *HostListPanel) SetFocused(focused bool) {
	p.focused = focused
}

// SetHosts はホスト一覧を設定する。
func (p *HostListPanel) SetHosts(hosts []core.SSHHost) {
	p.hosts = hosts
	if p.cursor >= len(hosts) {
		if len(hosts) > 0 {
			p.cursor = len(hosts) - 1
		} else {
			p.cursor = 0
		}
	}
}

// SetSize はパネルのサイズを設定する。
func (p *HostListPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SelectedHost は現在選択中のホストを返す。ホストがない場合は nil を返す。
func (p HostListPanel) SelectedHost() *core.SSHHost {
	if len(p.hosts) == 0 || p.cursor >= len(p.hosts) {
		return nil
	}
	h := p.hosts[p.cursor]
	return &h
}

// Update はキー入力を処理し、カーソル移動とホスト選択メッセージを発行する。
func (p HostListPanel) Update(msg tea.Msg) (HostListPanel, tea.Cmd) {
	if !p.focused {
		return p, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	keys := tui.DefaultKeyMap()
	prevCursor := p.cursor

	switch {
	case key.Matches(keyMsg, keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
	case key.Matches(keyMsg, keys.Down):
		if p.cursor < len(p.hosts)-1 {
			p.cursor++
		}
	default:
		return p, nil
	}

	if prevCursor != p.cursor && len(p.hosts) > 0 {
		host := p.hosts[p.cursor]
		return p, func() tea.Msg {
			return tui.HostSelectedMsg{Host: host}
		}
	}

	return p, nil
}

// View はパネルを描画する。
func (p HostListPanel) View() string {
	title := tui.TitleStyle.Render("SSH Hosts")

	// パネル内部幅（ボーダーとパディングを除く）
	innerWidth := p.width - 4 // ボーダー2 + パディング2
	if innerWidth < 10 {
		innerWidth = 10
	}

	divider := atoms.RenderDivider(innerWidth)

	var rows []string
	rows = append(rows, title)
	rows = append(rows, divider)

	if len(p.hosts) == 0 {
		rows = append(rows, tui.MutedStyle.Render("ホストが見つかりません"))
	} else {
		// 表示可能な行数を計算（タイトル + 区切り線 = 2行分を除く）
		maxRows := p.height - 4 // ボーダー2 + タイトル1 + 区切り線1
		if maxRows < 1 {
			maxRows = 1
		}

		// スクロールオフセットの計算
		offset := 0
		if p.cursor >= maxRows {
			offset = p.cursor - maxRows + 1
		}

		end := offset + maxRows
		if end > len(p.hosts) {
			end = len(p.hosts)
		}

		for i := offset; i < end; i++ {
			row := molecules.HostRow{
				Host:     p.hosts[i],
				Selected: i == p.cursor,
			}
			rows = append(rows, row.View())
		}
	}

	content := strings.Join(rows, "\n")

	style := tui.PanelBorder
	if p.focused {
		style = tui.PanelBorderFocused
	}

	return style.Width(innerWidth).Height(p.height - 2).Render(content)
}

// Cursor は現在のカーソル位置を返す。
func (p HostListPanel) Cursor() int {
	return p.cursor
}

// SetCursor はカーソル位置を設定する。
func (p *HostListPanel) SetCursor(cursor int) {
	if cursor >= 0 && cursor < len(p.hosts) {
		p.cursor = cursor
	}
}

// UpdateHostState は指定ホストの状態を更新する。
func (p *HostListPanel) UpdateHostState(hostName string, state core.ConnectionState) {
	for i := range p.hosts {
		if p.hosts[i].Name == hostName {
			p.hosts[i].State = state
			break
		}
	}
}

// Hosts は現在のホスト一覧を返す。
func (p HostListPanel) Hosts() []core.SSHHost {
	return p.hosts
}

// HostNames は全ホスト名のリストを返す。
func (p HostListPanel) HostNames() []string {
	names := make([]string, len(p.hosts))
	for i, h := range p.hosts {
		names[i] = h.Name
	}
	return names
}

