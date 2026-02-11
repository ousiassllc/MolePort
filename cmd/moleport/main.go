package main

import (
	"fmt"
	"os"

	"github.com/ousiassllc/moleport/internal/cli"
	"github.com/ousiassllc/moleport/internal/daemon"
)

func main() {
	// デーモンモードの場合は直接デーモンとして起動
	if daemon.IsDaemonMode() {
		flagConfigDir, _ := cli.ParseGlobalFlags()
		configDir := cli.ResolveConfigDir(flagConfigDir)
		cli.RunDaemonMode(configDir)
		return
	}

	// グローバルフラグを解析
	flagConfigDir, args := cli.ParseGlobalFlags()
	configDir := cli.ResolveConfigDir(flagConfigDir)

	// サブコマンドなしの場合は TUI を起動
	if len(args) == 0 {
		cli.RunTUI(configDir, nil)
		return
	}

	// サブコマンドをルーティング
	cmd := args[0]
	subArgs := args[1:]

	switch cmd {
	case "daemon":
		cli.RunDaemon(configDir, subArgs)
	case "connect":
		cli.RunConnect(configDir, subArgs)
	case "disconnect":
		cli.RunDisconnect(configDir, subArgs)
	case "add":
		cli.RunAdd(configDir, subArgs)
	case "delete":
		cli.RunDelete(configDir, subArgs)
	case "start":
		cli.RunStart(configDir, subArgs)
	case "stop":
		cli.RunStop(configDir, subArgs)
	case "list":
		cli.RunList(configDir, subArgs)
	case "status":
		cli.RunStatus(configDir, subArgs)
	case "config":
		cli.RunConfig(configDir, subArgs)
	case "reload":
		cli.RunReload(configDir, subArgs)
	case "tui":
		cli.RunTUI(configDir, subArgs)
	case "version":
		cli.RunVersion(configDir, subArgs)
	case "help", "--help", "-h":
		cli.RunHelp(configDir, subArgs)
	default:
		fmt.Fprintf(os.Stderr, "エラー: 不明なコマンド '%s'\n\n", cmd)
		cli.RunHelp(configDir, nil)
		os.Exit(1)
	}
}
