package cli

import "fmt"

const helpText = `MolePort - SSH ポートフォワーディングマネージャ

Usage:
  moleport <command> [arguments]

Commands:
  daemon start       デーモンをバックグラウンドで起動
  daemon stop [--purge]  デーモンを停止（--purge: 状態クリア）
  daemon status      デーモンの稼働状態を表示
  connect <host>     SSH ホストに接続
  disconnect <host>  SSH ホストを切断
  add [flags]        転送ルールを追加
  delete <name>      転送ルールを削除
  start <name>       フォワーディングを開始
  stop <name> / --all  フォワーディングを停止（--all: 全停止）
  list [--json]      ホスト・転送ルールの一覧
  status [name]      接続状態のサマリー
  config [--json]    設定を表示
  reload             SSH config を再読み込み
  tui                TUI ダッシュボードを起動
  help               このヘルプを表示
  version            バージョン情報を表示

Global Flags:
  --config-dir <path>  設定ディレクトリのパス`

// RunHelp は help サブコマンドを実行する。
func RunHelp(configDir string, args []string) {
	fmt.Println(helpText)
}
