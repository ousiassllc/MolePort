package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunDelete は delete サブコマンドを実行する。
func RunDelete(configDir string, args []string) {
	if len(args) == 0 {
		exitError("ルール名を指定してください: moleport delete <name>")
	}

	name := args[0]
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := protocol.ForwardDeleteParams{Name: name}
	var result protocol.ForwardDeleteResult
	if err := client.Call(ctx, "forward.delete", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Printf("ルール '%s' を削除しました\n", name)
}
