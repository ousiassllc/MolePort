package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunDisconnect は disconnect サブコマンドを実行する。
func RunDisconnect(configDir string, args []string) {
	if len(args) == 0 {
		exitError("ホスト名を指定してください: moleport disconnect <host>")
	}

	host := args[0]
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := ipc.SSHDisconnectParams{Host: host}
	var result ipc.SSHDisconnectResult
	if err := client.Call(ctx, "ssh.disconnect", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Printf("%s を切断しました\n", result.Host)
}
