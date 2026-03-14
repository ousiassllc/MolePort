package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// connectCallTimeout は ssh.connect 呼び出しのタイムアウト。
// クレデンシャル入力を待つため、通常の CallCtx より長くする。
const connectCallTimeout = 60 * time.Second

// RunConnect は connect サブコマンドを実行する。
func RunConnect(configDir string, args []string) {
	if len(args) == 0 {
		ExitError("%s", i18n.T("cli.connect.host_required"))
	}

	host := args[0]
	client := ConnectDaemon(configDir)
	defer client.Close()

	// クレデンシャルハンドラーを設定
	client.SetCredentialHandler(newCLICredentialHandler())

	ctx, cancel := context.WithTimeout(context.Background(), connectCallTimeout)
	defer cancel()

	params := protocol.SSHConnectParams{Host: host}
	var result protocol.SSHConnectResult
	if err := client.Call(ctx, "ssh.connect", params, &result); err != nil {
		ExitError("%v", err)
	}

	fmt.Println(i18n.T("cli.connect.success", map[string]any{"Host": result.Host}))
}
