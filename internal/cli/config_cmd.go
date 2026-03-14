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

	client, ctx, cleanup := DaemonCall(configDir)
	defer cleanup()

	var result protocol.ConfigGetResult
	if err := client.Call(ctx, "config.get", nil, &result); err != nil {
		ExitError("%s", i18n.T("cli.config.get_failed", map[string]any{"Error": err}))
	}

	if *jsonFlag {
		PrintJSON(result)
		return
	}

	fmt.Println(i18n.T("cli.config.header"))
	fmt.Println(i18n.T("cli.config.ssh_config", map[string]any{"Path": result.SSHConfigPath}))
	fmt.Println(i18n.T("cli.config.reconnect_header"))
	fmt.Println(i18n.T("cli.config.reconnect_enabled", map[string]any{"Value": result.Reconnect.Enabled}))
	fmt.Println(i18n.T("cli.config.reconnect_max_retries", map[string]any{"Value": result.Reconnect.MaxRetries}))
	fmt.Println(i18n.T("cli.config.reconnect_initial_delay", map[string]any{"Value": result.Reconnect.InitialDelay}))
	fmt.Println(i18n.T("cli.config.reconnect_max_delay", map[string]any{"Value": result.Reconnect.MaxDelay}))
	fmt.Println(i18n.T("cli.config.session_header"))
	fmt.Println(i18n.T("cli.config.session_auto_restore", map[string]any{"Value": result.Session.AutoRestore}))
	fmt.Println(i18n.T("cli.config.log_header"))
	fmt.Println(i18n.T("cli.config.log_level", map[string]any{"Value": result.Log.Level}))
	fmt.Println(i18n.T("cli.config.log_file", map[string]any{"Value": result.Log.File}))
}
