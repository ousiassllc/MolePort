package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunStart は start サブコマンドを実行する。
func RunStart(configDir string, args []string) {
	if len(args) == 0 {
		exitError("ルール名を指定してください: moleport start <name>")
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

	fmt.Printf("%s を開始しました\n", result.Name)
}
