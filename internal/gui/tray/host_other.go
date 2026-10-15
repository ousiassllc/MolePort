//go:build gui && !linux

package tray

// hostCheck は macOS / Windows ではトレイホストの確認が不要なため、required=false を返す。
func hostCheck() (required, present bool) { return false, false }
