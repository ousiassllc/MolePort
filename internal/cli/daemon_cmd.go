package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/infra"
	"github.com/ousiassllc/moleport/internal/infra/yamlstore"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunDaemon は daemon サブコマンドをルーティングする。
func RunDaemon(configDir string, args []string) {
	if len(args) == 0 {
		exitError("%s", i18n.T("cli.daemon.subcommand_required"))
	}

	switch args[0] {
	case "start":
		runDaemonStart(configDir)
	case "stop":
		runDaemonStop(configDir, args[1:])
	case "status":
		runDaemonStatus(configDir)
	case "kill":
		runDaemonKill(configDir)
	default:
		exitError("%s", i18n.T("cli.daemon.unknown_subcommand", map[string]any{"Sub": args[0]}))
	}
}

func runDaemonStart(configDir string) {
	pidPath := daemon.PIDFilePath(configDir)
	running, pid := daemon.IsRunning(pidPath)
	if running {
		fmt.Println(i18n.T("cli.daemon.already_running", map[string]any{"PID": pid}))
		return
	}

	pid, err := daemon.StartDaemonProcess(configDir)
	if err != nil {
		exitError("%s", i18n.T("cli.daemon.start_failed", map[string]any{"Error": err}))
	}

	fmt.Println(i18n.T("cli.daemon.started", map[string]any{"PID": pid}))
}

func runDaemonStop(configDir string, args []string) {
	fs := flag.NewFlagSet("daemon stop", flag.ContinueOnError)
	purge := fs.Bool("purge", false, "状態ファイルを削除して停止")
	if err := fs.Parse(args); err != nil {
		exitError("%v", err)
	}

	pidPath := daemon.PIDFilePath(configDir)
	running, _ := daemon.IsRunning(pidPath)
	if !running {
		fmt.Println(i18n.T("cli.daemon.not_running"))
		return
	}

	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		exitError("%s", i18n.T("cli.daemon.connect_failed", map[string]any{"Error": err}))
	}
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := protocol.DaemonShutdownParams{Purge: *purge}
	var result protocol.DaemonShutdownResult
	if err := client.Call(ctx, "daemon.shutdown", params, &result); err != nil {
		exitError("%s", i18n.T("cli.daemon.stop_failed", map[string]any{"Error": err}))
	}

	if *purge {
		fmt.Println(i18n.T("cli.daemon.stopped_purge"))
	} else {
		fmt.Println(i18n.T("cli.daemon.stopped"))
	}
}

func runDaemonKill(configDir string) {
	pidPath := daemon.PIDFilePath(configDir)
	running, pid := daemon.IsRunning(pidPath)
	if !running {
		fmt.Println(i18n.T("cli.daemon.not_running"))
		return
	}

	if err := daemon.KillProcess(pidPath); err != nil {
		exitError("%s", i18n.T("cli.daemon.kill_failed", map[string]any{"Error": err}))
	}

	// 強制終了では graceful shutdown が走らないため、state.yaml を手動で削除する。
	// 残すと次回起動時に古いセッションが自動復元されてしまう。
	statePath := filepath.Join(configDir, "state.yaml")
	_ = os.Remove(statePath)

	fmt.Println(i18n.T("cli.daemon.killed", map[string]any{"PID": pid}))
}

func runDaemonStatus(configDir string) {
	pidPath := daemon.PIDFilePath(configDir)
	running, _ := daemon.IsRunning(pidPath)
	if !running {
		fmt.Println(i18n.T("cli.daemon.not_running"))
		return
	}

	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		exitError("%s", i18n.T("cli.daemon.connect_failed", map[string]any{"Error": err}))
	}
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	var status protocol.DaemonStatusResult
	if err := client.Call(ctx, "daemon.status", nil, &status); err != nil {
		exitError("%s", i18n.T("cli.daemon.status_failed", map[string]any{"Error": err}))
	}

	fmt.Println("MolePort Daemon:")
	fmt.Printf("  Version:    %s\n", status.Version)
	fmt.Printf("  PID:        %d\n", status.PID)
	fmt.Printf("  Uptime:     %s\n", status.Uptime)
	fmt.Printf("  Clients:    %d connected\n", status.ConnectedClients)
	fmt.Printf("  SSH:        %d connections\n", status.ActiveSSHConnections)
	fmt.Printf("  Forwards:   %d active\n", status.ActiveForwards)
}

// RunDaemonMode はデーモンモードで起動する。
// --daemon-mode フラグが検出された場合に呼び出される。
func RunDaemonMode(configDir string) {
	logFile, err := setupDaemonLogging(configDir)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		exitFunc(1)
	}
	defer func() { _ = logFile.Close() }()

	d, err := daemon.New(configDir, Version)
	if err != nil {
		slog.Error("failed to create daemon", "error", err)
		exitFunc(1)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		slog.Error("failed to start daemon", "error", err)
		exitFunc(1)
	}

	if err := d.Wait(); err != nil {
		slog.Error("daemon error", "error", err)
		exitFunc(1)
	}
}

// setupDaemonLogging はデーモンプロセス用のログ設定を行う。
// 設定ファイルの log.file と log.level を参照する。
// ログファイルの作成に失敗した場合はエラーを返す。
func setupDaemonLogging(configDir string) (*os.File, error) {
	store := yamlstore.NewYAMLStore()
	cfgMgr := core.NewConfigManager(store, configDir)
	cfg, err := cfgMgr.LoadConfig()
	if err != nil {
		c := core.DefaultConfig()
		cfg = &c
	}

	logPath := cfg.Log.File
	if expanded, err := infra.ExpandTilde(logPath); err == nil {
		logPath = expanded
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	level := parseSlogLevel(cfg.Log.Level)
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
	return f, nil
}

// parseSlogLevel は文字列を slog.Level に変換する。
func parseSlogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
