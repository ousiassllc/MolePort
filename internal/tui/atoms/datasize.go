package atoms

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/tui"
)

// RenderDataSize はバイト数を人間可読な文字列として描画する。
func RenderDataSize(bytes int64) string {
	var text string
	switch {
	case bytes >= 1<<30:
		text = fmt.Sprintf("%.1fGB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		text = fmt.Sprintf("%.1fMB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		text = fmt.Sprintf("%.1fKB", float64(bytes)/float64(1<<10))
	default:
		text = fmt.Sprintf("%dB", bytes)
	}
	return tui.MutedStyle.Render(text)
}
