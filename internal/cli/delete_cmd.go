package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunDelete は delete サブコマンドを実行する。
func RunDelete(configDir string, args []string) {
	if len(args) == 0 {
		ExitError("%s", i18n.T("cli.delete.name_required"))
	}

	name := args[0]
	client, ctx, cleanup := DaemonCall(configDir)
	defer cleanup()

	params := protocol.ForwardDeleteParams{Name: name}
	var result protocol.ForwardDeleteResult
	if err := client.Call(ctx, "forward.delete", params, &result); err != nil {
		ExitError("%v", err)
	}

	fmt.Println(i18n.T("cli.delete.success", map[string]any{"Name": name}))
}
