package protocol

// --- SSH 接続管理 ---

// SSHConnectParams は ssh.connect リクエストのパラメータ。
type SSHConnectParams struct {
	Host string `json:"host"`
}

// SSHConnectResult は ssh.connect リクエストの結果。
type SSHConnectResult struct {
	Host   string `json:"host"`
	Status string `json:"status"`
}

// SSHDisconnectParams は ssh.disconnect リクエストのパラメータ。
type SSHDisconnectParams struct {
	Host string `json:"host"`
}

// SSHDisconnectResult は ssh.disconnect リクエストの結果。
type SSHDisconnectResult struct {
	Host   string `json:"host"`
	Status string `json:"status"`
}
