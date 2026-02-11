package cli

import "fmt"

// RunTUI は tui サブコマンドを実行する。
// 現在はスタブ実装で、次のスコープで TUI モードが実装される予定。
func RunTUI(configDir string, args []string) {
	fmt.Println("TUI モードは次のバージョンで実装予定です。")
	fmt.Println("現在は CLI サブコマンドをご利用ください。moleport help で一覧を確認できます。")
}
