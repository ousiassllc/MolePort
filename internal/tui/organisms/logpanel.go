package organisms

import (
	"strings"

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
	// innerWidth = p.width - 4 (2 border + 2 padding)
	innerWidth := p.width - 4
	if innerWidth < 10 {
		innerWidth = 10
	}
	// innerHeight = p.height - 2 (top + bottom border)
	innerHeight := p.height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	// 出力バッファから表示分を取得
	var lines []string
	if len(p.output) > innerHeight {
		lines = p.output[len(p.output)-innerHeight:]
	} else {
		lines = p.output
	}

	// 不足分の空行で埋める
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}

	var rows []string
	for _, line := range lines {
		rows = append(rows, styleLogLine(line))
	}

	content := strings.Join(rows, "\n")
	return tui.RenderWithBorderTitle(tui.UnfocusedBorder(), innerWidth, innerHeight, "Log", content)
}

// styleLogLine はログ行にスタイルを適用する。
func styleLogLine(line string) string {
	if line == "" {
		return ""
	}
	if isErrorLine(line) {
		return tui.ErrorStyle().Render("✗") + " " + tui.MutedStyle().Render(line)
	}
	if isSuccessLine(line) {
		return tui.ActiveStyle().Render("✓") + " " + tui.MutedStyle().Render(line)
	}
	return tui.MutedStyle().Render(line)
}

// エラー判定キーワード（日本語 + 英語 + slog レベルプレフィックス）。
var errorKeywords = []string{
	"エラー", "失敗",
	"error", "Error", "ERROR",
	"failed", "Failed",
}

// 成功判定キーワード（日本語 + 英語）。
var successKeywords = []string{
	"しました", "完了", "復元",
	"started", "stopped", "added", "deleted",
	"loaded", "reloaded", "restarted", "saved",
}

func isErrorLine(line string) bool {
	for _, kw := range errorKeywords {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}

func isSuccessLine(line string) bool {
	for _, kw := range successKeywords {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}
