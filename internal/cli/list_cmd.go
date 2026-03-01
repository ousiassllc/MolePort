package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunList は list サブコマンドを実行する。
func RunList(configDir string, args []string) {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON 形式で出力")
	hostFlag := fs.String("host", "", "特定ホストのルールのみ表示")

	if err := fs.Parse(args); err != nil {
		exitError("%v", err)
	}

	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	// ホスト一覧を取得
	var hosts protocol.HostListResult
	if err := client.Call(ctx, "host.list", nil, &hosts); err != nil {
		exitError("%s", i18n.T("cli.list.get_hosts_failed", map[string]any{"Error": err}))
	}

	// フォワードルール一覧を取得
	fwdParams := protocol.ForwardListParams{Host: *hostFlag}
	var forwards protocol.ForwardListResult
	if err := client.Call(ctx, "forward.list", fwdParams, &forwards); err != nil {
		exitError("%s", i18n.T("cli.list.get_forwards_failed", map[string]any{"Error": err}))
	}

	if *jsonFlag {
		printJSON(struct {
			Hosts    []protocol.HostInfo    `json:"hosts"`
			Forwards []protocol.ForwardInfo `json:"forwards"`
		}{
			Hosts:    hosts.Hosts,
			Forwards: forwards.Forwards,
		})
		return
	}

	// ホスト数と接続数をカウント
	connectedCount := 0
	for _, h := range hosts.Hosts {
		if h.State == "connected" {
			connectedCount++
		}
	}

	fmt.Println(i18n.T("cli.list.hosts_header", map[string]any{"Total": len(hosts.Hosts), "Connected": connectedCount}))
	fmt.Println()

	// ホスト別に転送ルールをまとめて表示
	fwdByHost := make(map[string][]protocol.ForwardInfo)
	for _, f := range forwards.Forwards {
		fwdByHost[f.Host] = append(fwdByHost[f.Host], f)
	}

	for _, h := range hosts.Hosts {
		if *hostFlag != "" && h.Name != *hostFlag {
			continue
		}

		icon := "○"
		switch h.State {
		case "connected":
			icon = "●"
		case "pending_auth":
			icon = "◎"
		}

		fmt.Printf("%s %s (%s:%d, %s)\n", icon, h.Name, h.HostName, h.Port, h.User)

		rules := fwdByHost[h.Name]
		if len(rules) == 0 {
			fmt.Println("  " + i18n.T("cli.list.no_rules"))
		} else {
			for _, f := range rules {
				printForwardLine(f)
			}
		}
		fmt.Println()
	}
}

func printForwardLine(f protocol.ForwardInfo) {
	typeChar := "L"
	switch f.Type {
	case "remote":
		typeChar = "R"
	case "dynamic":
		typeChar = "D"
	}

	if f.Type == "dynamic" {
		fmt.Printf("  %s  :%d\n", typeChar, f.LocalPort)
	} else {
		fmt.Printf("  %s  :%d  ->  %s:%d\n", typeChar, f.LocalPort, f.RemoteHost, f.RemotePort)
	}
}
