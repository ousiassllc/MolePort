package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunStart は start サブコマンドを実行する。
func RunStart(configDir string, args []string) {
	if len(args) == 0 {
		exitError("%s", i18n.T("cli.start.name_required"))
	}

	name := args[0]
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := protocol.ForwardStartParams{Name: name}
	var result protocol.ForwardStartResult
	if err := client.Call(ctx, "forward.start", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Println(i18n.T("cli.start.success", map[string]any{"Name": result.Name}))
}
