package format

import (
	"fmt"
	"time"
)

// Duration は経過時間を "2d 3h" / "2h 15m" / "5m 30s" / "12s" 形式の文字列に変換する。
// d は 0 以上を前提とする。負の値が渡された場合は 0 秒として扱う。
func Duration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}
