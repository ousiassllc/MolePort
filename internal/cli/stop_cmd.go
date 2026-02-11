package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunStop は stop サブコマンドを実行する。
func RunStop(configDir string, args []string) {
	if len(args) == 0 {
		exitError("ルール名を指定してください: moleport stop <name>")
	}

	name := args[0]
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := ipc.ForwardStopParams{Name: name}
	var result ipc.ForwardStopResult
	if err := client.Call(ctx, "forward.stop", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Printf("%s を停止しました\n", result.Name)
}
