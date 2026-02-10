package atoms

import (
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/tui"
)

// RenderDuration は経過時間を人間可読な文字列として描画する。
func RenderDuration(d time.Duration) string {
	var text string
	switch {
	case d >= 24*time.Hour:
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		text = fmt.Sprintf("%dd %dh", days, hours)
	case d >= time.Hour:
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		text = fmt.Sprintf("%dh %dm", hours, minutes)
	case d >= time.Minute:
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		text = fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		text = fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return tui.MutedStyle.Render(text)
}
