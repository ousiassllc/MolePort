package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunAdd は add サブコマンドを実行する。
func RunAdd(configDir string, args []string) {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)

	host := fs.String("host", "", "SSH ホスト名 (必須)")
	fwdType := fs.String("type", "local", "転送種別: local, remote, dynamic")
	localPort := fs.Int("local-port", 0, "ローカルポート (必須)")
	remoteHost := fs.String("remote-host", "localhost", "リモートホスト")
	remotePort := fs.Int("remote-port", 0, "リモートポート")
	name := fs.String("name", "", "ルール名 (省略時は自動生成)")
	autoConnect := fs.Bool("auto-connect", false, "起動時に自動接続")

	if err := fs.Parse(args); err != nil {
		exitError("%v", err)
	}

	if *host == "" {
		exitError("--host フラグは必須です")
	}
	if *localPort == 0 {
		exitError("--local-port フラグは必須です")
	}
	if *localPort < 1 || *localPort > 65535 {
		exitError("ポート番号は 1〜65535 の範囲で入力してください")
	}

	switch *fwdType {
	case "local", "remote", "dynamic":
		// OK
	default:
		exitError("--type は local, remote, dynamic のいずれかを指定してください")
	}

	if *fwdType != "dynamic" {
		if *remotePort == 0 {
			exitError("--remote-port フラグは local/remote 転送で必須です")
		}
		if *remotePort < 1 || *remotePort > 65535 {
			exitError("ポート番号は 1〜65535 の範囲で入力してください")
		}
	}

	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := ipc.ForwardAddParams{
		Name:        *name,
		Host:        *host,
		Type:        *fwdType,
		LocalPort:   *localPort,
		RemoteHost:  *remoteHost,
		RemotePort:  *remotePort,
		AutoConnect: *autoConnect,
	}

	var result ipc.ForwardAddResult
	if err := client.Call(ctx, "forward.add", params, &result); err != nil {
		exitError("%v", err)
	}

	fmt.Printf("ルール '%s' を追加しました\n", result.Name)
}
