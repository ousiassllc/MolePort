package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
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
	var hosts ipc.HostListResult
	if err := client.Call(ctx, "host.list", nil, &hosts); err != nil {
		exitError("ホスト一覧の取得に失敗しました: %v", err)
	}

	// フォワードルール一覧を取得
	fwdParams := ipc.ForwardListParams{Host: *hostFlag}
	var forwards ipc.ForwardListResult
	if err := client.Call(ctx, "forward.list", fwdParams, &forwards); err != nil {
		exitError("転送ルール一覧の取得に失敗しました: %v", err)
	}

	if *jsonFlag {
		printJSON(struct {
			Hosts    []ipc.HostInfo    `json:"hosts"`
			Forwards []ipc.ForwardInfo `json:"forwards"`
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

	fmt.Printf("SSH Hosts (%d hosts, %d connected):\n\n", len(hosts.Hosts), connectedCount)

	// ホスト別に転送ルールをまとめて表示
	fwdByHost := make(map[string][]ipc.ForwardInfo)
	for _, f := range forwards.Forwards {
		fwdByHost[f.Host] = append(fwdByHost[f.Host], f)
	}

	for _, h := range hosts.Hosts {
		if *hostFlag != "" && h.Name != *hostFlag {
			continue
		}

		icon := "○"
		if h.State == "connected" {
			icon = "●"
		}

		fmt.Printf("%s %s (%s:%d, %s)\n", icon, h.Name, h.HostName, h.Port, h.User)

		rules := fwdByHost[h.Name]
		if len(rules) == 0 {
			fmt.Println("  (転送ルールなし)")
		} else {
			for _, f := range rules {
				printForwardLine(f)
			}
		}
		fmt.Println()
	}
}

func printForwardLine(f ipc.ForwardInfo) {
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
