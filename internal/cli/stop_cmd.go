package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunStop は stop サブコマンドを実行する。
func RunStop(configDir string, args []string) {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	all := fs.Bool("all", false, "全フォワーディングを一括停止")
	if err := fs.Parse(args); err != nil {
		ExitError("%v", err)
	}

	client := ConnectDaemon(configDir)
	defer client.Close()

	ctx, cancel := CallCtx()
	defer cancel()

	if *all {
		var result protocol.ForwardStopAllResult
		if err := client.Call(ctx, "forward.stopAll", nil, &result); err != nil {
			ExitError("%v", err)
		}
		fmt.Println(i18n.T("cli.stop.all_stopped", map[string]any{"Count": result.Stopped}))
		return
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		ExitError("%s", i18n.T("cli.stop.name_required"))
	}

	name := remaining[0]
	params := protocol.ForwardStopParams{Name: name}
	var result protocol.ForwardStopResult
	if err := client.Call(ctx, "forward.stop", params, &result); err != nil {
		ExitError("%v", err)
	}

	fmt.Println(i18n.T("cli.stop.success", map[string]any{"Name": result.Name}))
}
