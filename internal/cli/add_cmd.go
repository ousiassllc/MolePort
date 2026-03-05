package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
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
		ExitError("%v", err)
	}

	if *host == "" {
		ExitError("%s", i18n.T("cli.add.host_required"))
	}
	if *localPort == 0 {
		ExitError("%s", i18n.T("cli.add.local_port_required"))
	}
	if *localPort < 1 || *localPort > 65535 {
		ExitError("%s", i18n.T("cli.add.port_range"))
	}

	switch *fwdType {
	case "local", "remote", "dynamic":
		// OK
	default:
		ExitError("%s", i18n.T("cli.add.type_invalid"))
	}

	if *fwdType != "dynamic" {
		if *remotePort == 0 {
			ExitError("%s", i18n.T("cli.add.remote_port_required"))
		}
		if *remotePort < 1 || *remotePort > 65535 {
			ExitError("%s", i18n.T("cli.add.port_range"))
		}
	}

	client := ConnectDaemon(configDir)
	defer client.Close()

	ctx, cancel := CallCtx()
	defer cancel()

	params := protocol.ForwardAddParams{
		Name:        *name,
		Host:        *host,
		Type:        *fwdType,
		LocalPort:   *localPort,
		RemoteHost:  *remoteHost,
		RemotePort:  *remotePort,
		AutoConnect: *autoConnect,
	}

	var result protocol.ForwardAddResult
	if err := client.Call(ctx, "forward.add", params, &result); err != nil {
		ExitError("%v", err)
	}

	fmt.Println(i18n.T("cli.add.success", map[string]any{"Name": result.Name}))
}
