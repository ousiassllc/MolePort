package organisms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tui "github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/theme"
)

// ThemeGrid はテーマプリセットを2カラム（Dark/Light）で表示・選択するコンポーネント。
type ThemeGrid struct {
	darkPresets  []theme.Preset
	lightPresets []theme.Preset
	baseIndex    int // 0=Dark, 1=Light
	accentIndex  int // 各カラム内のインデックス
	width        int
	height       int
}

// NewThemeGrid は指定されたプリセット ID から初期カーソル位置を復元した ThemeGrid を返す。
func NewThemeGrid(currentPresetID string) ThemeGrid {
	g := ThemeGrid{
		darkPresets:  theme.PresetsByBase("dark"),
		lightPresets: theme.PresetsByBase("light"),
	}

	p, ok := theme.FindPreset(currentPresetID)
	if !ok {
		return g // デフォルト: baseIndex=0, accentIndex=0
	}

	if p.Base == "light" {
		g.baseIndex = 1
	}

	presets := g.activePresets()
	for i, pr := range presets {
		if pr.ID == currentPresetID {
			g.accentIndex = i
			break
		}
	}

	return g
}

// Update はキー入力に応じてカーソルを移動し、リアルタイムプレビューを適用する。
func (g ThemeGrid) Update(msg tea.Msg) (ThemeGrid, tea.Cmd) {
	km := tui.DefaultKeyMap()
	leftKey := key.NewBinding(key.WithKeys("left", "h"))
	rightKey := key.NewBinding(key.WithKeys("right", "l"))

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return g, nil
	}

	switch {
	case key.Matches(keyMsg, km.Up):
		if g.accentIndex > 0 {
			g.accentIndex--
			g.applySelected()
		}
	case key.Matches(keyMsg, km.Down):
		if g.accentIndex < g.activeColumnLen()-1 {
			g.accentIndex++
			g.applySelected()
		}
	case key.Matches(keyMsg, leftKey):
		if g.baseIndex > 0 {
			g.baseIndex--
			g.accentIndex = g.clampedAccentIndex()
			g.applySelected()
		}
	case key.Matches(keyMsg, rightKey):
		if g.baseIndex < 1 {
			g.baseIndex++
			g.accentIndex = g.clampedAccentIndex()
			g.applySelected()
		}
	}

	return g, nil
}

// SelectedPresetID は現在選択されているプリセットの ID を返す。
func (g ThemeGrid) SelectedPresetID() string {
	presets := g.activePresets()
	if g.accentIndex < len(presets) {
		return presets[g.accentIndex].ID
	}
	return theme.DefaultPresetID()
}

// SetSize はグリッドの描画サイズを設定する。
func (g *ThemeGrid) SetSize(width, height int) {
	g.width = width
	g.height = height
}

// View はテーマグリッドを描画する。
func (g ThemeGrid) View() string {
	columnWidth := (g.width - 2) / 2
	if columnWidth < 20 {
		columnWidth = 20
	}

	left := g.renderColumn("Dark", g.darkPresets, 0, columnWidth)
	right := g.renderColumn("Light", g.lightPresets, 1, columnWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (g ThemeGrid) renderColumn(title string, presets []theme.Preset, baseIdx, width int) string {
	innerWidth := width - 4 // ボーダー2 + パディング2
	if innerWidth < 10 {
		innerWidth = 10
	}
	innerHeight := len(presets)

	var rows []string
	for i, p := range presets {
		selected := g.baseIndex == baseIdx && g.accentIndex == i
		rows = append(rows, g.renderPresetRow(p, selected))
	}

	content := strings.Join(rows, "\n")

	var style lipgloss.Style
	if g.baseIndex == baseIdx {
		style = tui.FocusedBorder()
	} else {
		style = tui.UnfocusedBorder()
	}

	return tui.RenderWithBorderTitle(style, innerWidth, innerHeight, title, content)
}

func (g ThemeGrid) renderPresetRow(preset theme.Preset, selected bool) string {
	swatch := lipgloss.NewStyle().Foreground(preset.Palette.Accent).Render("●")

	if selected {
		label := lipgloss.NewStyle().Bold(true).Foreground(preset.Palette.Accent).Render(preset.Label)
		return "> " + swatch + " " + label
	}
	return "  " + swatch + " " + preset.Label
}

// activePresets は現在選択されているカラムのプリセットリストを返す。
func (g ThemeGrid) activePresets() []theme.Preset {
	if g.baseIndex == 1 {
		return g.lightPresets
	}
	return g.darkPresets
}

// activeColumnLen は現在カラムのプリセット数を返す。
func (g ThemeGrid) activeColumnLen() int {
	return len(g.activePresets())
}

// clampedAccentIndex はカラム切替時に accentIndex を有効範囲に補正した値を返す。
func (g ThemeGrid) clampedAccentIndex() int {
	maxIdx := g.activeColumnLen() - 1
	if maxIdx < 0 {
		return 0
	}
	if g.accentIndex > maxIdx {
		return maxIdx
	}
	return g.accentIndex
}

// applySelected は現在選択されているプリセットをテーマに適用する。
func (g ThemeGrid) applySelected() {
	theme.Apply(g.SelectedPresetID())
}
