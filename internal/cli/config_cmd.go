package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunConfig は config サブコマンドを実行する。
func RunConfig(configDir string, args []string) {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON 形式で出力")

	if err := fs.Parse(args); err != nil {
		exitError("%v", err)
	}

	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	var result ipc.ConfigGetResult
	if err := client.Call(ctx, "config.get", nil, &result); err != nil {
		exitError("設定の取得に失敗しました: %v", err)
	}

	if *jsonFlag {
		printJSON(result)
		return
	}

	fmt.Println("MolePort Config:")
	fmt.Printf("  SSH Config:     %s\n", result.SSHConfigPath)
	fmt.Println("  Reconnect:")
	fmt.Printf("    Enabled:      %v\n", result.Reconnect.Enabled)
	fmt.Printf("    Max Retries:  %d\n", result.Reconnect.MaxRetries)
	fmt.Printf("    Initial Delay: %s\n", result.Reconnect.InitialDelay)
	fmt.Printf("    Max Delay:    %s\n", result.Reconnect.MaxDelay)
	fmt.Println("  Session:")
	fmt.Printf("    Auto Restore: %v\n", result.Session.AutoRestore)
	fmt.Println("  Log:")
	fmt.Printf("    Level:        %s\n", result.Log.Level)
	fmt.Printf("    File:         %s\n", result.Log.File)
}
