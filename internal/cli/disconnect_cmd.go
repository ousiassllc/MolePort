package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunDisconnect は disconnect サブコマンドを実行する。
func RunDisconnect(configDir string, args []string) {
	if len(args) == 0 {
		ExitError("%s", i18n.T("cli.disconnect.host_required"))
	}

	host := args[0]
	client, ctx, cleanup := DaemonCall(configDir)
	defer cleanup()

	params := protocol.SSHDisconnectParams{Host: host}
	var result protocol.SSHDisconnectResult
	if err := client.Call(ctx, "ssh.disconnect", params, &result); err != nil {
		ExitError("%v", err)
	}

	fmt.Println(i18n.T("cli.disconnect.success", map[string]any{"Host": result.Host}))
}
