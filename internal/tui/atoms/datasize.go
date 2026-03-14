package atoms

import (
	"github.com/ousiassllc/moleport/internal/format"
	"github.com/ousiassllc/moleport/internal/tui"
)

// RenderDataSize はバイト数を人間可読な文字列として描画する。
func RenderDataSize(bytes int64) string {
	return tui.MutedStyle().Render(format.Bytes(bytes))
}

// RenderTraffic は送受信トラフィックを ↑/↓ シンボル付きで描画する。
func RenderTraffic(sent, received int64) string {
	up := tui.DividerStyle().Render("↑") + RenderDataSize(sent)
	down := tui.DividerStyle().Render("↓") + RenderDataSize(received)
	return up + " " + down
}
