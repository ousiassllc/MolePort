package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunConfig は config サブコマンドを実行する。
func RunConfig(configDir string, args []string) {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON 形式で出力")

	if err := fs.Parse(args); err != nil {
		ExitError("%v", err)
	}

	client := ConnectDaemon(configDir)
	defer client.Close()

	ctx, cancel := CallCtx()
	defer cancel()

	var result protocol.ConfigGetResult
	if err := client.Call(ctx, "config.get", nil, &result); err != nil {
		ExitError("%s", i18n.T("cli.config.get_failed", map[string]any{"Error": err}))
	}

	if *jsonFlag {
		PrintJSON(result)
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
