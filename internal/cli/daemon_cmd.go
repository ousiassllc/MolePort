package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunDaemon は daemon サブコマンドをルーティングする。
func RunDaemon(configDir string, args []string) {
	if len(args) == 0 {
		exitError("サブコマンドを指定してください: start, stop, status")
	}

	switch args[0] {
	case "start":
		runDaemonStart(configDir)
	case "stop":
		runDaemonStop(configDir, args[1:])
	case "status":
		runDaemonStatus(configDir)
	default:
		exitError("不明なサブコマンド: daemon %s", args[0])
	}
}

func runDaemonStart(configDir string) {
	pidPath := daemon.PIDFilePath(configDir)
	running, pid := daemon.IsRunning(pidPath)
	if running {
		fmt.Printf("デーモンは既に稼働中です (PID: %d)\n", pid)
		return
	}

	pid, err := daemon.StartDaemonProcess(configDir)
	if err != nil {
		exitError("デーモンの起動に失敗しました: %v", err)
	}

	fmt.Printf("デーモンを起動しました (PID: %d)\n", pid)
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
		fmt.Println("デーモンは稼働していません")
		return
	}

	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		exitError("デーモンへの接続に失敗しました: %v", err)
	}
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := ipc.DaemonShutdownParams{Purge: *purge}
	var result ipc.DaemonShutdownResult
	if err := client.Call(ctx, "daemon.shutdown", params, &result); err != nil {
		exitError("デーモンの停止に失敗しました: %v", err)
	}

	if *purge {
		fmt.Println("デーモンを停止しました（状態をクリア）")
	} else {
		fmt.Println("デーモンを停止しました")
	}
}

func runDaemonStatus(configDir string) {
	pidPath := daemon.PIDFilePath(configDir)
	running, _ := daemon.IsRunning(pidPath)
	if !running {
		fmt.Println("デーモンは稼働していません")
		return
	}

	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		exitError("デーモンへの接続に失敗しました: %v", err)
	}
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	var status ipc.DaemonStatusResult
	if err := client.Call(ctx, "daemon.status", nil, &status); err != nil {
		exitError("ステータスの取得に失敗しました: %v", err)
	}

	fmt.Println("MolePort Daemon:")
	fmt.Printf("  PID:        %d\n", status.PID)
	fmt.Printf("  Uptime:     %s\n", status.Uptime)
	fmt.Printf("  Clients:    %d connected\n", status.ConnectedClients)
	fmt.Printf("  SSH:        %d connections\n", status.ActiveSSHConnections)
	fmt.Printf("  Forwards:   %d active\n", status.ActiveForwards)
}

// RunDaemonMode はデーモンモードで起動する。
// --daemon-mode フラグが検出された場合に呼び出される。
func RunDaemonMode(configDir string) {
	if err := setupDaemonLogging(configDir); err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}

	d, err := daemon.New(configDir)
	if err != nil {
		slog.Error("failed to create daemon", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		slog.Error("failed to start daemon", "error", err)
		os.Exit(1)
	}

	if err := d.Wait(); err != nil {
		slog.Error("daemon error", "error", err)
		os.Exit(1)
	}
}

// setupDaemonLogging はデーモンプロセス用のログ設定を行う。
// ログファイルの作成に失敗した場合はエラーを返す。
func setupDaemonLogging(configDir string) error {
	logPath := filepath.Join(configDir, "daemon.log")

	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))
	return nil
}
