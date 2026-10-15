//go:build gui

// moleport-gui は Wails ベースのデスクトップ GUI。Core と IPC サーバーを同一プロセスに内蔵する。
// ビルドには gui タグと OS ネイティブの WebView 環境が必要（make gui-build）。
package main

import (
	"fmt"
	"os"

	"github.com/ousiassllc/moleport/internal/cli"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/shell"
	"github.com/ousiassllc/moleport/internal/gui/web/static"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/infra/yamlstore"
	"github.com/ousiassllc/moleport/internal/version"
)

func main() {
	flagConfigDir, _ := cli.ParseGlobalFlags()
	configDir := cli.ResolveConfigDir(flagConfigDir)
	initI18n(configDir)

	if err := shell.Run(shell.Config{
		ConfigDir: configDir,
		Version:   version.Version,
		Static:    static.FS(),
	}); err != nil {
		fmt.Fprintln(os.Stderr, "moleport-gui:", err)
		os.Exit(1)
	}
}

// initI18n は config.yaml の Language を読んで i18n を初期化する。
// 読み込みに失敗した場合は環境変数にフォールバックする（cmd/moleport と同じ挙動）。
func initI18n(configDir string) {
	var configLang string
	cfgMgr := core.NewConfigManager(yamlstore.NewYAMLStore(), configDir)
	if cfg, err := cfgMgr.LoadConfig(); err == nil {
		configLang = cfg.Language
	}
	_ = i18n.SetLang(i18n.Resolve(configLang))
}
