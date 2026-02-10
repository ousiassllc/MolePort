package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/infra"
	"github.com/ousiassllc/moleport/internal/tui/app"
)

func main() {
	configDir := defaultConfigDir()

	store := infra.NewYAMLStore()
	configMgr := core.NewConfigManager(store, configDir)
	cfg, err := configMgr.LoadConfig()
	if err != nil {
		// 設定ファイルが存在しない場合はデフォルト設定を使用
		c := core.DefaultConfig()
		cfg = &c
	}

	setupLogging(cfg.Log)

	// SSH config パスの ~ を展開
	sshConfigPath := cfg.SSHConfigPath
	if expanded, err := infra.ExpandTilde(sshConfigPath); err == nil {
		sshConfigPath = expanded
	}

	parser := infra.NewSSHConfigParser()
	sshMgr := core.NewSSHManager(
		parser,
		func() core.SSHConnection { return infra.NewSSHConnection() },
		sshConfigPath,
		cfg.Reconnect,
	)
	fwdMgr := core.NewForwardManager(sshMgr)

	// 保存済みのフォワードルールを読み込む
	for _, rule := range cfg.Forwards {
		if err := fwdMgr.AddRule(rule); err != nil {
			slog.Warn("failed to load forward rule", "rule", rule.Name, "error", err)
		}
	}

	model := app.NewMainModel(sshMgr, fwdMgr, configMgr)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// defaultConfigDir はデフォルトの設定ディレクトリパスを返す。
func defaultConfigDir() string {
	// XDG_CONFIG_HOME を優先
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "moleport")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "moleport")
}

// setupLogging はログ設定を適用する。
func setupLogging(logCfg core.LogConfig) {
	level := slog.LevelInfo
	switch logCfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	logPath := logCfg.File
	if expanded, err := infra.ExpandTilde(logPath); err == nil {
		logPath = expanded
	}

	// ログファイルの親ディレクトリを作成
	if logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0700); err == nil {
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err == nil {
				handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
				slog.SetDefault(slog.New(handler))
				return
			}
		}
	}

	// ファイルに書き込めない場合は stderr にフォールバック
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
