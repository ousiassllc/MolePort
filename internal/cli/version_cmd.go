package cli

import (
	"fmt"
	"os"
	"runtime"

	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// Version はビルド時に -ldflags で設定されるバージョン情報。
var Version = "dev"

// RunVersion は version サブコマンドを実行する。
func RunVersion(configDir string, args []string) {
	fmt.Printf("MolePort %s (%s, %s/%s)\n", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// デーモンが稼働中ならバージョンチェックを実行
	pidPath := daemon.PIDFilePath(configDir)
	running, _ := daemon.IsRunning(pidPath)
	if !running {
		return
	}

	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := callCtx()
	defer cancel()

	var result protocol.VersionCheckResult
	if err := client.Call(ctx, "version.check", protocol.VersionCheckParams{}, &result); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("cli.version.check_failed", map[string]any{"Error": err}))
		return
	}

	if result.UpdateAvailable {
		fmt.Println(i18n.T("cli.version.update_available", map[string]any{
			"Latest":  result.LatestVersion,
			"Current": result.CurrentVersion,
		}))
		if result.ReleaseURL != "" {
			fmt.Println(i18n.T("cli.version.release_url", map[string]any{
				"URL": result.ReleaseURL,
			}))
		}
	}
}
