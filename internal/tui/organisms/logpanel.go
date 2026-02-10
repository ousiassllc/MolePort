package organisms

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/tui"
)

const logMaxOutputLines = 100

// LogPanel はログ/出力メッセージを表示する読み取り専用パネル。
type LogPanel struct {
	output []string
	width  int
	height int
}

// NewLogPanel は新しい LogPanel を生成する。
func NewLogPanel() LogPanel {
	return LogPanel{}
}

// AppendOutput は出力バッファにテキストを追加する。
func (p *LogPanel) AppendOutput(text string) {
	lines := strings.Split(text, "\n")
	p.output = append(p.output, lines...)
	if len(p.output) > logMaxOutputLines {
		p.output = p.output[len(p.output)-logMaxOutputLines:]
	}
}

// SetSize はパネルのサイズを設定する。
func (p *LogPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// View はパネルを描画する。
func (p LogPanel) View() string {
	contentWidth := p.width
	if contentWidth < 10 {
		contentWidth = 10
	}

	displayLines := p.height
	if displayLines < 1 {
		displayLines = 1
	}

	// 出力バッファから表示分を取得
	var lines []string
	if len(p.output) > displayLines {
		lines = p.output[len(p.output)-displayLines:]
	} else {
		lines = p.output
	}

	// 不足分の空行で埋める
	for len(lines) < displayLines {
		lines = append(lines, "")
	}

	var rows []string
	for _, line := range lines {
		styled := styleLogLine(line)
		rows = append(rows, "  "+styled)
	}

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(contentWidth).Height(p.height).Render(content)
}

// styleLogLine はログ行にスタイルを適用する。
func styleLogLine(line string) string {
	if line == "" {
		return ""
	}
	// エラー行
	if strings.Contains(line, "エラー") || strings.Contains(line, "Error") {
		return tui.ErrorStyle.Render("✗") + " " + tui.MutedStyle.Render(line)
	}
	// 成功行（「しました」「完了」等）
	if strings.Contains(line, "しました") || strings.Contains(line, "完了") || strings.Contains(line, "復元") {
		return tui.ActiveStyle.Render("✓") + " " + tui.MutedStyle.Render(line)
	}
	return tui.MutedStyle.Render(line)
}
