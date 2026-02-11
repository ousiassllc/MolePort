package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunConnect は connect サブコマンドを実行する。
func RunConnect(configDir string, args []string) {
	if len(args) == 0 {
		exitError("ホスト名を指定してください: moleport connect <host>")
	}

	host := args[0]
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := ipc.SSHConnectParams{Host: host}
	var result ipc.SSHConnectResult
	if err := client.Call(ctx, "ssh.connect", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Printf("%s に接続しました\n", result.Host)
}
