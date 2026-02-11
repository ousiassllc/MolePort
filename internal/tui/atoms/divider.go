package atoms

import (
	"strings"

	"github.com/ousiassllc/moleport/internal/tui"
)

// RenderDivider は指定幅の水平区切り線を描画する。
func RenderDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return tui.DividerStyle.Render(strings.Repeat("─", width))
}
