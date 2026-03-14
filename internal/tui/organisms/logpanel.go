package organisms

import (
	"strings"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/tui"
)

const logMaxOutputLines = 100

type logEntry struct {
	text  string
	level tui.LogLevel
}

// LogPanel はログ/出力メッセージを表示する読み取り専用パネル。
type LogPanel struct {
	output []logEntry
	width  int
	height int
}

// NewLogPanel は新しい LogPanel を生成する。
func NewLogPanel() LogPanel {
	return LogPanel{}
}

// AppendOutput は出力バッファにテキストを追加する。
func (p *LogPanel) AppendOutput(text string, level tui.LogLevel) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		p.output = append(p.output, logEntry{text: line, level: level})
	}
	if len(p.output) > logMaxOutputLines {
		p.output = p.output[len(p.output)-logMaxOutputLines:]
	}
}

// OutputLen は出力バッファの行数を返す。
func (p LogPanel) OutputLen() int {
	return len(p.output)
}

// SetSize はパネルのサイズを設定する。
func (p *LogPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// View はパネルを描画する。
func (p LogPanel) View() string {
	innerWidth, innerHeight := panelInnerSize(p.width, p.height)

	var entries []logEntry
	if len(p.output) > innerHeight {
		entries = p.output[len(p.output)-innerHeight:]
	} else {
		entries = p.output
	}

	for len(entries) < innerHeight {
		entries = append(entries, logEntry{})
	}

	var rows []string
	for _, entry := range entries {
		rows = append(rows, styleLogEntry(entry))
	}

	content := strings.Join(rows, "\n")
	return tui.RenderWithBorderTitle(tui.UnfocusedBorder(), innerWidth, innerHeight, i18n.T("tui.log.title"), content)
}

func styleLogEntry(entry logEntry) string {
	if entry.text == "" {
		return ""
	}
	switch entry.level {
	case tui.LogError:
		return tui.ErrorStyle().Render("✗") + " " + tui.MutedStyle().Render(entry.text)
	case tui.LogSuccess:
		return tui.ActiveStyle().Render("✓") + " " + tui.MutedStyle().Render(entry.text)
	default:
		return tui.MutedStyle().Render(entry.text)
	}
}
