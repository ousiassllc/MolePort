package atoms

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/tui"
)

// RenderPortLabel はポート番号をフォーマットされた文字列として描画する。
func RenderPortLabel(port int) string {
	return tui.TitleStyle.Render(fmt.Sprintf(":%d", port))
}
