package pages

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/i18n"
	tui "github.com/ousiassllc/moleport/internal/tui"
)

// LangPage は言語選択ページ。
type LangPage struct {
	langs  []i18n.LangInfo
	cursor int
	keys   tui.KeyMap
	width  int
	height int
}

// NewLangPage は新しい LangPage を生成する。
func NewLangPage(currentLang string) LangPage {
	langs := i18n.SupportedLangs()
	cursor := 0
	for i, l := range langs {
		if string(l.Code) == currentLang {
			cursor = i
			break
		}
	}
	return LangPage{
		langs:  langs,
		cursor: cursor,
		keys:   tui.DefaultKeyMap(),
	}
}

// Init は Bubble Tea の Init メソッド。
func (p LangPage) Init() tea.Cmd { return nil }

// Update はメッセージを処理する。
func (p LangPage) Update(msg tea.Msg) (LangPage, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, p.keys.Enter) {
			lang := string(p.langs[p.cursor].Code)
			return p, func() tea.Msg {
				return tui.LangSelectedMsg{Lang: lang}
			}
		}
		if key.Matches(msg, p.keys.Escape) {
			return p, func() tea.Msg {
				return tui.LangCancelledMsg{}
			}
		}
		if key.Matches(msg, p.keys.Up) {
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		}
		if key.Matches(msg, p.keys.Down) {
			if p.cursor < len(p.langs)-1 {
				p.cursor++
			}
			return p, nil
		}
	}
	return p, nil
}

// View は言語選択ページを描画する。
func (p LangPage) View() string {
	header := tui.HeaderStyle().Render("  " + i18n.T("tui.setup.lang_title"))

	var items []string
	for i, l := range p.langs {
		prefix := "    "
		if i == p.cursor {
			prefix = "  > "
		}
		line := prefix + l.Label
		if i == p.cursor {
			line = tui.SelectedStyle().Render(line)
		}
		items = append(items, line)
	}
	list := lipgloss.JoinVertical(lipgloss.Left, items...)

	help := tui.MutedStyle().Render("  " + i18n.T("tui.lang.help"))
	return lipgloss.JoinVertical(lipgloss.Left, header, "", list, "", help)
}

// SetSize はページのサイズを設定する。
func (p *LangPage) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SelectedLang は現在選択中の言語コードを返す。
func (p LangPage) SelectedLang() string {
	return string(p.langs[p.cursor].Code)
}
