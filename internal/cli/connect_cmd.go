package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// connectCallTimeout は ssh.connect 呼び出しのタイムアウト。
// クレデンシャル入力を待つため、通常の CallCtx より長くする。
const connectCallTimeout = 60 * time.Second

// DaemonCallWithCredentials はデーモンに接続し、クレデンシャルハンドラーを設定した上で
// connectCallTimeout 付きのコンテキストとクリーンアップ関数を返す。
// ssh.connect や forward.start のように、認証入力を求められうる RPC 呼び出しに使う。
func DaemonCallWithCredentials(configDir string) (cl *client.IPCClient, ctx context.Context, cleanup func()) {
	cl = ConnectDaemon(configDir)
	cl.SetCredentialHandler(newCLICredentialHandler())
	ctx, cancel := context.WithTimeout(context.Background(), connectCallTimeout)
	cleanup = func() {
		cancel()
		_ = cl.Close()
	}
	return
}

// RunConnect は connect サブコマンドを実行する。
func RunConnect(configDir string, args []string) {
	if len(args) == 0 {
		ExitError("%s", i18n.T("cli.connect.host_required"))
	}

	host := args[0]
	client, ctx, cleanup := DaemonCallWithCredentials(configDir)
	defer cleanup()

	params := protocol.SSHConnectParams{Host: host}
	var result protocol.SSHConnectResult
	if err := client.Call(ctx, "ssh.connect", params, &result); err != nil {
		ExitError("connect failed: %v", err)
	}

	fmt.Println(i18n.T("cli.connect.success", map[string]any{"Host": result.Host}))
}
