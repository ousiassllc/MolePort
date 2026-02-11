package atoms

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/tui"
)

// RenderPortLabel はポート番号をアクセントカラーでフォーマットされた文字列として描画する。
func RenderPortLabel(port int) string {
	return tui.ActiveStyle.Render(fmt.Sprintf(":%d", port))
}
