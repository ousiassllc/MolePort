package daemoncmd

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ousiassllc/moleport/internal/cli"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunDaemon は daemon サブコマンドをルーティングする。
func RunDaemon(configDir string, args []string) {
	if len(args) == 0 {
		cli.ExitError("%s", i18n.T("cli.daemon.subcommand_required"))
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
		cli.ExitError("%s", i18n.T("cli.daemon.unknown_subcommand", map[string]any{"Sub": args[0]}))
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
		cli.ExitError("%s", i18n.T("cli.daemon.start_failed", map[string]any{"Error": err}))
	}

	fmt.Println(i18n.T("cli.daemon.started", map[string]any{"PID": pid}))
}

func runDaemonStop(configDir string, args []string) {
	fs := flag.NewFlagSet("daemon stop", flag.ContinueOnError)
	purge := fs.Bool("purge", false, "状態ファイルを削除して停止")
	if err := fs.Parse(args); err != nil {
		cli.ExitError("%v", err)
	}

	pidPath := daemon.PIDFilePath(configDir)
	running, _ := daemon.IsRunning(pidPath)
	if !running {
		fmt.Println(i18n.T("cli.daemon.not_running"))
		return
	}

	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		cli.ExitError("%s", i18n.T("cli.daemon.connect_failed", map[string]any{"Error": err}))
	}
	defer client.Close()

	ctx, cancel := cli.CallCtx()
	defer cancel()

	params := protocol.DaemonShutdownParams{Purge: *purge}
	var result protocol.DaemonShutdownResult
	if err := client.Call(ctx, "daemon.shutdown", params, &result); err != nil {
		cli.ExitError("%s", i18n.T("cli.daemon.stop_failed", map[string]any{"Error": err}))
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
		cli.ExitError("%s", i18n.T("cli.daemon.kill_failed", map[string]any{"Error": err}))
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
		cli.ExitError("%s", i18n.T("cli.daemon.connect_failed", map[string]any{"Error": err}))
	}
	defer client.Close()

	ctx, cancel := cli.CallCtx()
	defer cancel()

	var status protocol.DaemonStatusResult
	if err := client.Call(ctx, "daemon.status", nil, &status); err != nil {
		cli.ExitError("%s", i18n.T("cli.daemon.status_failed", map[string]any{"Error": err}))
	}

	fmt.Println(i18n.T("cli.daemon.status_header"))
	fmt.Println(i18n.T("cli.daemon.status_version", map[string]any{"Version": status.Version}))
	fmt.Println(i18n.T("cli.daemon.status_pid", map[string]any{"PID": status.PID}))
	fmt.Println(i18n.T("cli.daemon.status_uptime", map[string]any{"Uptime": status.Uptime}))
	fmt.Println(i18n.T("cli.daemon.status_clients", map[string]any{"Count": status.ConnectedClients}))
	fmt.Println(i18n.T("cli.daemon.status_ssh", map[string]any{"Count": status.ActiveSSHConnections}))
	fmt.Println(i18n.T("cli.daemon.status_forwards", map[string]any{"Count": status.ActiveForwards}))
}

// RunDaemonMode はデーモンモードで起動する。
// --daemon-mode フラグが検出された場合に呼び出される。
func RunDaemonMode(configDir string) {
	logFile, err := setupDaemonLogging(configDir)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		cli.ExitFunc(1)
	}
	defer func() { _ = logFile.Close() }()

	d, err := daemon.New(configDir, cli.Version)
	if err != nil {
		slog.Error("failed to create daemon", "error", err)
		cli.ExitFunc(1)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		slog.Error("failed to start daemon", "error", err)
		cli.ExitFunc(1)
	}

	if err := d.Wait(); err != nil {
		slog.Error("daemon error", "error", err)
		cli.ExitFunc(1)
	}
}

// setupDaemonLogging はデーモンプロセス用のログ設定を行う。
func setupDaemonLogging(configDir string) (*os.File, error) {
	logCfg := daemon.ResolveLogConfig(configDir)

	if err := os.MkdirAll(filepath.Dir(logCfg.Path), 0700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	f, err := os.OpenFile(logCfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	level := parseSlogLevel(logCfg.Level)
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
