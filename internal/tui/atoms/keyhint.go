package atoms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/ousiassllc/moleport/internal/tui"
)

// RenderKeyHint はキーバインドヒントを "[key] description" 形式で描画する。
func RenderKeyHint(bindings ...key.Binding) string {
	var parts []string
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		keys := b.Help().Key
		desc := b.Help().Desc
		part := tui.KeyStyle.Render("["+keys+"]") + " " + tui.DescStyle.Render(desc)
		parts = append(parts, part)
	}
	return strings.Join(parts, "  ")
}
