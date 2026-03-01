package cli

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunStatus は status サブコマンドを実行する。
func RunStatus(configDir string, args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON 形式で出力")

	if err := fs.Parse(args); err != nil {
		exitError("%v", err)
	}

	remaining := fs.Args()

	// 名前が指定された場合はセッション詳細を表示
	if len(remaining) > 0 {
		runSessionGet(configDir, remaining[0], *jsonFlag)
		return
	}

	// 名前なしの場合はサマリーを表示
	runStatusSummary(configDir, *jsonFlag)
}

func runSessionGet(configDir string, name string, jsonOutput bool) {
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	params := protocol.SessionGetParams{Name: name}
	var session protocol.SessionGetResult
	if err := client.Call(ctx, "session.get", params, &session); err != nil {
		exitError("%v", err)
	}

	if jsonOutput {
		printJSON(session)
		return
	}

	fmt.Printf("Session: %s\n", session.Name)
	fmt.Printf("  Host:           %s\n", session.Host)
	fmt.Printf("  Type:           %s\n", session.Type)
	fmt.Printf("  Local Port:     %d\n", session.LocalPort)
	if session.RemoteHost != "" {
		fmt.Printf("  Remote:         %s:%d\n", session.RemoteHost, session.RemotePort)
	}
	fmt.Printf("  Status:         %s\n", session.Status)
	if session.ConnectedAt != "" {
		fmt.Printf("  Connected At:   %s\n", session.ConnectedAt)
	}
	fmt.Printf("  Bytes Sent:     %s\n", formatBytes(session.BytesSent))
	fmt.Printf("  Bytes Received: %s\n", formatBytes(session.BytesReceived))
	if session.ReconnectCount > 0 {
		fmt.Printf("  Reconnects:     %d\n", session.ReconnectCount)
	}
	if session.LastError != "" {
		fmt.Printf("  Last Error:     %s\n", session.LastError)
	}
}

func runStatusSummary(configDir string, jsonOutput bool) {
	pidPath := daemon.PIDFilePath(configDir)
	running, _ := daemon.IsRunning(pidPath)
	if !running {
		fmt.Println(i18n.T("cli.daemon.not_running"))
		return
	}

	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		exitError("%s", i18n.T("cli.daemon.connect_failed", map[string]any{"Error": err}))
	}
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	// デーモンステータス
	var daemonStatus protocol.DaemonStatusResult
	if err := client.Call(ctx, "daemon.status", nil, &daemonStatus); err != nil {
		exitError("%s", i18n.T("cli.status.get_failed", map[string]any{"Error": err}))
	}

	// ホスト一覧
	var hosts protocol.HostListResult
	if err := client.Call(ctx, "host.list", nil, &hosts); err != nil {
		exitError("%s", i18n.T("cli.status.get_hosts_failed", map[string]any{"Error": err}))
	}

	// セッション一覧
	var sessions protocol.SessionListResult
	if err := client.Call(ctx, "session.list", nil, &sessions); err != nil {
		exitError("%s", i18n.T("cli.status.get_sessions_failed", map[string]any{"Error": err}))
	}

	if jsonOutput {
		printJSON(struct {
			Daemon   protocol.DaemonStatusResult `json:"daemon"`
			Hosts    []protocol.HostInfo         `json:"hosts"`
			Sessions []protocol.SessionInfo      `json:"sessions"`
		}{
			Daemon:   daemonStatus,
			Hosts:    hosts.Hosts,
			Sessions: sessions.Sessions,
		})
		return
	}

	connectedHosts := 0
	pendingAuthHosts := 0
	for _, h := range hosts.Hosts {
		switch h.State {
		case "connected":
			connectedHosts++
		case "pending_auth":
			pendingAuthHosts++
		}
	}

	activeSessions := 0
	stoppedSessions := 0
	var totalSent, totalRecv int64
	for _, s := range sessions.Sessions {
		if s.Status == "active" {
			activeSessions++
		} else {
			stoppedSessions++
		}
		totalSent += s.BytesSent
		totalRecv += s.BytesReceived
	}

	fmt.Println("MolePort Status:")
	fmt.Printf("  Daemon:    Running (PID: %d, uptime: %s)\n", daemonStatus.PID, daemonStatus.Uptime)
	if pendingAuthHosts > 0 {
		fmt.Printf("  Hosts:     %d total, %d connected, %d pending auth\n", len(hosts.Hosts), connectedHosts, pendingAuthHosts)
	} else {
		fmt.Printf("  Hosts:     %d total, %d connected\n", len(hosts.Hosts), connectedHosts)
	}
	fmt.Printf("  Forwards:  %d total, %d active, %d stopped\n", len(sessions.Sessions), activeSessions, stoppedSessions)
	fmt.Printf("  Traffic:   sent %s, recv %s\n", formatBytes(totalSent), formatBytes(totalRecv))
}

// formatBytes はバイト数を人間が読みやすい形式に変換する。
func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case b >= GB:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
