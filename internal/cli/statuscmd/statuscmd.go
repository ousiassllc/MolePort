package statuscmd

import (
	"flag"
	"fmt"

	"github.com/ousiassllc/moleport/internal/cli"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/format"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// RunStatus は status サブコマンドを実行する。
func RunStatus(configDir string, args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "JSON 形式で出力")

	if err := fs.Parse(args); err != nil {
		cli.ExitError("%v", err)
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
	client := cli.ConnectDaemon(configDir)
	defer client.Close()

	ctx, cancel := cli.CallCtx()
	defer cancel()

	params := protocol.SessionGetParams{Name: name}
	var session protocol.SessionGetResult
	if err := client.Call(ctx, "session.get", params, &session); err != nil {
		cli.ExitError("%v", err)
	}

	if jsonOutput {
		cli.PrintJSON(session)
		return
	}

	fmt.Println(i18n.T("cli.status.session_header", map[string]any{"Name": session.Name}))
	fmt.Println(i18n.T("cli.status.session_host", map[string]any{"Host": session.Host}))
	fmt.Println(i18n.T("cli.status.session_type", map[string]any{"Type": session.Type}))
	fmt.Println(i18n.T("cli.status.session_local_port", map[string]any{"Port": session.LocalPort}))
	if session.RemoteHost != "" {
		fmt.Println(i18n.T("cli.status.session_remote", map[string]any{"Remote": fmt.Sprintf("%s:%d", session.RemoteHost, session.RemotePort)}))
	}
	fmt.Println(i18n.T("cli.status.session_status", map[string]any{"Status": session.Status}))
	if session.ConnectedAt != "" {
		fmt.Println(i18n.T("cli.status.session_connected_at", map[string]any{"Time": session.ConnectedAt}))
	}
	fmt.Println(i18n.T("cli.status.session_bytes_sent", map[string]any{"Bytes": format.Bytes(session.BytesSent)}))
	fmt.Println(i18n.T("cli.status.session_bytes_received", map[string]any{"Bytes": format.Bytes(session.BytesReceived)}))
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
		cli.ExitError("%s", i18n.T("cli.daemon.connect_failed", map[string]any{"Error": err}))
	}
	defer client.Close()

	ctx, cancel := cli.CallCtx()
	defer cancel()

	// デーモンステータス
	var daemonStatus protocol.DaemonStatusResult
	if err := client.Call(ctx, "daemon.status", nil, &daemonStatus); err != nil {
		cli.ExitError("%s", i18n.T("cli.status.get_failed", map[string]any{"Error": err}))
	}

	// ホスト一覧
	var hosts protocol.HostListResult
	if err := client.Call(ctx, "host.list", nil, &hosts); err != nil {
		cli.ExitError("%s", i18n.T("cli.status.get_hosts_failed", map[string]any{"Error": err}))
	}

	// セッション一覧
	var sessions protocol.SessionListResult
	if err := client.Call(ctx, "session.list", nil, &sessions); err != nil {
		cli.ExitError("%s", i18n.T("cli.status.get_sessions_failed", map[string]any{"Error": err}))
	}

	if jsonOutput {
		cli.PrintJSON(struct {
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
		case protocol.StateConnected:
			connectedHosts++
		case protocol.StatePendingAuth:
			pendingAuthHosts++
		}
	}

	activeSessions := 0
	stoppedSessions := 0
	var totalSent, totalRecv int64
	for _, s := range sessions.Sessions {
		if s.Status == protocol.SessionActive {
			activeSessions++
		} else {
			stoppedSessions++
		}
		totalSent += s.BytesSent
		totalRecv += s.BytesReceived
	}

	fmt.Println(i18n.T("cli.status.header"))
	fmt.Println(i18n.T("cli.status.daemon_running", map[string]any{"PID": daemonStatus.PID, "Uptime": daemonStatus.Uptime}))
	if pendingAuthHosts > 0 {
		fmt.Println(i18n.T("cli.status.hosts_summary_auth", map[string]any{"Total": len(hosts.Hosts), "Connected": connectedHosts, "PendingAuth": pendingAuthHosts}))
	} else {
		fmt.Println(i18n.T("cli.status.hosts_summary", map[string]any{"Total": len(hosts.Hosts), "Connected": connectedHosts}))
	}
	fmt.Println(i18n.T("cli.status.forwards_summary", map[string]any{"Total": len(sessions.Sessions), "Active": activeSessions, "Stopped": stoppedSessions}))
	fmt.Println(i18n.T("cli.status.traffic_summary", map[string]any{"Sent": format.Bytes(totalSent), "Received": format.Bytes(totalRecv)}))
}
