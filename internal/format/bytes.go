package format

import "fmt"

// Bytes はバイト数を人間が読みやすい形式に変換する。
// b は 0 以上を前提とする。負の値が渡された場合の出力は未定義。
func Bytes(b int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
