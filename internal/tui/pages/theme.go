package pages

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/i18n"
	tui "github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/organisms"
)

// ThemePage はテーマ選択ページ。
type ThemePage struct {
	grid   organisms.ThemeGrid
	keys   tui.KeyMap
	width  int
	height int
}

// NewThemePage は新しい ThemePage を生成する。
func NewThemePage(currentPresetID string) ThemePage {
	return ThemePage{
		grid: organisms.NewThemeGrid(currentPresetID),
		keys: tui.DefaultKeyMap(),
	}
}

// Init は Bubble Tea の Init メソッド。
func (p ThemePage) Init() tea.Cmd { return nil }

// Update はメッセージを処理する。
func (p ThemePage) Update(msg tea.Msg) (ThemePage, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, p.keys.Enter) {
			presetID := p.grid.SelectedPresetID()
			return p, func() tea.Msg {
				return tui.ThemeSelectedMsg{PresetID: presetID}
			}
		}
		if key.Matches(msg, p.keys.Escape) {
			return p, func() tea.Msg {
				return tui.ThemeCancelledMsg{}
			}
		}
		// ThemeGrid にキーを転送
		var cmd tea.Cmd
		p.grid, cmd = p.grid.Update(msg)
		return p, cmd
	}
	return p, nil
}

// View はテーマ選択ページを描画する。
func (p ThemePage) View() string {
	header := tui.HeaderStyle().Render("  " + i18n.T("tui.theme.header"))
	gridView := p.grid.View()
	help := tui.MutedStyle().Render("  " + i18n.T("tui.theme.help"))
	return lipgloss.JoinVertical(lipgloss.Left, header, "", gridView, "", help)
}

// SetSize はページのサイズを設定する。
func (p *ThemePage) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.grid.SetSize(width, height-4) // ヘッダー+余白+ヘルプ+余白分
}
