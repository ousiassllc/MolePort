package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunReload は reload サブコマンドを実行する。
func RunReload(configDir string, args []string) {
	client, ctx, cleanup := DaemonCall(configDir)
	defer cleanup()

	var result protocol.HostReloadResult
	if err := client.Call(ctx, "host.reload", nil, &result); err != nil {
		ExitError("%s", i18n.T("cli.reload.failed", map[string]any{"Error": err}))
	}

	fmt.Println(i18n.T("cli.reload.success"))
	fmt.Println(i18n.T("cli.reload.hosts_count", map[string]any{
		"Total": result.Total, "Added": len(result.Added), "Removed": len(result.Removed),
	}))

	for _, name := range result.Added {
		fmt.Println(i18n.T("cli.reload.host_added", map[string]any{"Name": name}))
	}
	for _, name := range result.Removed {
		fmt.Println(i18n.T("cli.reload.host_removed", map[string]any{"Name": name}))
	}
}
