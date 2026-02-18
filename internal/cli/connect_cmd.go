package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// connectCallTimeout は ssh.connect 呼び出しのタイムアウト。
// クレデンシャル入力を待つため、通常の callCtx より長くする。
const connectCallTimeout = 60 * time.Second

// RunConnect は connect サブコマンドを実行する。
func RunConnect(configDir string, args []string) {
	if len(args) == 0 {
		exitError("ホスト名を指定してください: moleport connect <host>")
	}

	host := args[0]
	client := connectDaemon(configDir)
	defer client.Close()

	// クレデンシャルハンドラーを設定
	client.SetCredentialHandler(newCLICredentialHandler())

	ctx, cancel := context.WithTimeout(context.Background(), connectCallTimeout)
	defer cancel()

	params := ipc.SSHConnectParams{Host: host}
	var result ipc.SSHConnectResult
	if err := client.Call(ctx, "ssh.connect", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Printf("%s に接続しました\n", result.Host)
}
