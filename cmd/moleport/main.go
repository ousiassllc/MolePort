package main

import (
	"fmt"
	"os"

	"github.com/ousiassllc/moleport/internal/cli"
	"github.com/ousiassllc/moleport/internal/cli/daemoncmd"
	"github.com/ousiassllc/moleport/internal/cli/statuscmd"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/infra/yamlstore"
)

func main() {
	// デーモンモードの場合は直接デーモンとして起動
	if daemon.IsDaemonMode() {
		flagConfigDir, _ := cli.ParseGlobalFlags()
		configDir := cli.ResolveConfigDir(flagConfigDir)
		daemoncmd.RunDaemonMode(configDir)
		return
	}

	// グローバルフラグを解析
	flagConfigDir, args := cli.ParseGlobalFlags()
	configDir := cli.ResolveConfigDir(flagConfigDir)
	initI18n(configDir)

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
		daemoncmd.RunDaemon(configDir, subArgs)
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
		statuscmd.RunStatus(configDir, subArgs)
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
		fmt.Fprintln(os.Stderr, i18n.T("cli.error.unknown_command", map[string]any{"Command": cmd}))
		fmt.Fprintln(os.Stderr)
		cli.RunHelp(configDir, nil)
		os.Exit(1)
	}
}

// initI18n は config.yaml の Language フィールドを読み取り、i18n を初期化する。
// config の読み込みに失敗した場合は環境変数からフォールバックする。
func initI18n(configDir string) {
	var configLang string
	store := yamlstore.NewYAMLStore()
	cfgMgr := core.NewConfigManager(store, configDir)
	if cfg, err := cfgMgr.LoadConfig(); err == nil {
		configLang = cfg.Language
	}
	lang := i18n.Resolve(configLang)
	_ = i18n.SetLang(lang)
}
