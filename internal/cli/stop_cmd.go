package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunStop は stop サブコマンドを実行する。
func RunStop(configDir string, args []string) {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	all := fs.Bool("all", false, "全フォワーディングを一括停止")
	if err := fs.Parse(args); err != nil {
		exitError("%v", err)
	}

	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	if *all {
		var result ipc.ForwardStopAllResult
		if err := client.Call(ctx, "forward.stopAll", nil, &result); err != nil {
			exitError("%v", err)
		}
		fmt.Printf("全フォワーディングを停止しました (%d 件)\n", result.Stopped)
		return
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		exitError("ルール名を指定してください: moleport stop <name> / --all")
	}

	name := remaining[0]
	params := ipc.ForwardStopParams{Name: name}
	var result ipc.ForwardStopResult
	if err := client.Call(ctx, "forward.stop", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Printf("%s を停止しました\n", result.Name)
}
