package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunDisconnect は disconnect サブコマンドを実行する。
func RunDisconnect(configDir string, args []string) {
	if len(args) == 0 {
		exitError("%s", i18n.T("cli.disconnect.host_required"))
	}

	host := args[0]
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := protocol.SSHDisconnectParams{Host: host}
	var result protocol.SSHDisconnectResult
	if err := client.Call(ctx, "ssh.disconnect", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Println(i18n.T("cli.disconnect.success", map[string]any{"Host": result.Host}))
}
