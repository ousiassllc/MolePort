package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/ousiassllc/moleport/internal/core/update"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunUpdate は update サブコマンドを実行する。
func RunUpdate(configDir string, args []string) {
	checkOnly := false
	for _, a := range args {
		if a == "--check" {
			checkOnly = true
		}
	}

	// dev ビルドではアップデート不可
	if Version == "dev" {
		ExitError("%s", i18n.T("cli.update.dev_build"))
	}

	fmt.Println(i18n.T("cli.update.checking"))

	ctx := context.Background()
	vc := update.New(Version, true, 0)

	result, err := vc.LatestVersion(ctx)
	if err != nil {
		ExitError("%s", i18n.T("cli.update.check_failed", map[string]any{"Error": err}))
	}

	if result == nil || !result.UpdateAvailable {
		fmt.Println(i18n.T("cli.update.already_latest", map[string]any{"Version": Version}))
		return
	}

	fmt.Println(i18n.T("cli.update.available", map[string]any{
		"Latest":  result.LatestVersion,
		"Current": Version,
	}))

	if checkOnly {
		if result.ReleaseURL != "" {
			fmt.Println("  " + result.ReleaseURL)
		}
		return
	}

	// フルアップデートフロー
	u := update.NewUpdater(vc)

	pidPath := daemon.PIDFilePath(configDir)
	daemonRunning, _ := daemon.IsRunning(pidPath)

	assetName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	fmt.Println(i18n.T("cli.update.downloading", map[string]any{"Asset": assetName}))

	progress := func(stage string) {
		switch stage {
		case "verifying":
			fmt.Println(i18n.T("cli.update.verifying"))
		case "replacing":
			if daemonRunning {
				fmt.Println(i18n.T("cli.update.stopping_daemon"))
				stopDaemonForUpdate(configDir)
			}
			fmt.Println(i18n.T("cli.update.replacing"))
		}
	}

	if err := u.Update(ctx, progress); err != nil {
		if daemonRunning {
			restartDaemonAfterUpdate(configDir)
		}
		ExitError("%s", i18n.T("cli.update.failed", map[string]any{"Error": err}))
	}

	if daemonRunning {
		fmt.Println(i18n.T("cli.update.restarting_daemon"))
		restartDaemonAfterUpdate(configDir)
	}

	fmt.Println(i18n.T("cli.update.success", map[string]any{"Version": result.LatestVersion}))
}

// stopDaemonForUpdate はアップデート前にデーモンを停止する。
func stopDaemonForUpdate(configDir string) {
	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		return
	}

	ctx, cancel := CallCtx()
	defer cancel()

	var shutdownResult protocol.DaemonShutdownResult
	_ = client.Call(ctx, "daemon.shutdown", protocol.DaemonShutdownParams{}, &shutdownResult)
	_ = client.Close()
}

// restartDaemonAfterUpdate はアップデート後にデーモンを再起動する。
func restartDaemonAfterUpdate(configDir string) {
	if _, err := daemon.StartDaemonProcess(configDir); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cli.update.restart_failed"))
	}
}
