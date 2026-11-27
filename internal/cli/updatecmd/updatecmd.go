package updatecmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/ousiassllc/moleport/internal/cli"
	"github.com/ousiassllc/moleport/internal/core/update"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/daemon/pidfile"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/version"
)

// newVersionChecker は VersionChecker を生成するファクトリ関数。テストで差し替え可能。
// NOTE: stubVersionChecker を使用するテストは t.Parallel() と併用不可。
var newVersionChecker = func(version string) *update.VersionChecker {
	return update.New(version, true, 0)
}

// テストで差し替え可能な外部依存。
// NOTE: これらを差し替えるテストは t.Parallel() と併用不可。
var (
	isDaemonRunning = func(configDir string) bool {
		running, _ := pidfile.IsRunning(daemon.PIDFilePath(configDir))
		return running
	}
	takeoverDaemon     = daemon.Takeover
	startDaemonProcess = daemon.StartDaemonProcess
	applyUpdate        = func(ctx context.Context, vc *update.VersionChecker, beforeReplace func() error, progress func(string)) error {
		u := update.NewUpdater(vc)
		u.SetBeforeReplace(beforeReplace)
		return u.Update(ctx, progress)
	}
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
	if version.Version == "dev" {
		cli.ExitError("%s", i18n.T("cli.update.dev_build"))
	}

	fmt.Println(i18n.T("cli.update.checking"))

	ctx := context.Background()
	vc := newVersionChecker(version.Version)

	result, err := vc.LatestVersion(ctx)
	if err != nil {
		cli.ExitError("%s", i18n.T("cli.update.check_failed", map[string]any{"Error": err}))
	}

	if result == nil || !result.UpdateAvailable {
		fmt.Println(i18n.T("cli.update.already_latest", map[string]any{"Version": version.Version}))
		return
	}

	fmt.Println(i18n.T("cli.update.available", map[string]any{
		"Latest":  result.LatestVersion,
		"Current": version.Version,
	}))

	if checkOnly {
		if result.ReleaseURL != "" {
			fmt.Println("  " + result.ReleaseURL)
		}
		return
	}

	// フルアップデートフロー
	daemonRunning := isDaemonRunning(configDir)

	assetName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	fmt.Println(i18n.T("cli.update.downloading", map[string]any{"Asset": assetName}))

	// デーモンはバイナリ置換の直前（ダウンロード・検証が済んだ後）に停止する。
	// 停止に成功した場合のみ、更新後（失敗時も）に再起動する。
	daemonStopped := false
	var stopErr error
	beforeReplace := func() error {
		if !daemonRunning {
			return nil
		}
		fmt.Println(i18n.T("cli.update.stopping_daemon"))
		if stopErr = stopDaemonForUpdate(configDir); stopErr != nil {
			return stopErr
		}
		daemonStopped = true
		return nil
	}

	progress := func(stage string) {
		switch stage {
		case "verifying":
			fmt.Println(i18n.T("cli.update.verifying"))
		case "replacing":
			fmt.Println(i18n.T("cli.update.replacing"))
		}
	}

	if err := applyUpdate(ctx, vc, beforeReplace, progress); err != nil {
		if daemonStopped {
			restartDaemonAfterUpdate(configDir)
		}
		if stopErr != nil {
			cli.ExitError("%s", i18n.T("cli.update.stop_failed", map[string]any{"Error": stopErr}))
		}
		cli.ExitError("%s", i18n.T("cli.update.failed", map[string]any{"Error": err}))
	}

	if daemonStopped {
		fmt.Println(i18n.T("cli.update.restarting_daemon"))
		restartDaemonAfterUpdate(configDir)
	}

	fmt.Println(i18n.T("cli.update.success", map[string]any{"Version": result.LatestVersion}))
}

// stopDaemonForUpdate はアップデート前にデーモンを停止し、PID ロックの解放まで待つ。
// 停止できなかった場合（応答なし・時間切れ）はエラーを返す。
func stopDaemonForUpdate(configDir string) error {
	_, err := takeoverDaemon(configDir, daemon.TakeoverTimeout)
	return err
}

// restartDaemonAfterUpdate はアップデート後にデーモンを再起動する。
func restartDaemonAfterUpdate(configDir string) {
	if _, err := startDaemonProcess(configDir); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", i18n.T("cli.update.restart_failed"))
	}
}
