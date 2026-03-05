package protocol

// --- デーモン管理 ---

// DaemonStatusParams は daemon.status リクエストのパラメータ。
type DaemonStatusParams struct{}

// DaemonStatusResult は daemon.status リクエストの結果。
type DaemonStatusResult struct {
	Version              string   `json:"version"`
	PID                  int      `json:"pid"`
	StartedAt            string   `json:"started_at"`
	Uptime               string   `json:"uptime"`
	ConnectedClients     int      `json:"connected_clients"`
	ActiveSSHConnections int      `json:"active_ssh_connections"`
	ActiveForwards       int      `json:"active_forwards"`
	Warnings             []string `json:"warnings,omitempty"`
}

// DaemonShutdownParams は daemon.shutdown リクエストのパラメータ。
type DaemonShutdownParams struct {
	Purge bool `json:"purge,omitempty"`
}

// DaemonShutdownResult は daemon.shutdown リクエストの結果。
type DaemonShutdownResult struct {
	OK bool `json:"ok"`
}
