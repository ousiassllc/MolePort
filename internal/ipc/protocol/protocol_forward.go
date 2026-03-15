package protocol

// --- ポートフォワーディング管理 ---

// ForwardListParams は forward.list リクエストのパラメータ。
type ForwardListParams struct {
	Host string `json:"host,omitempty"`
}

// ForwardListResult は forward.list リクエストの結果。
type ForwardListResult struct {
	Forwards []ForwardInfo `json:"forwards"`
}

// ForwardInfo はポートフォワーディングルールの情報を表す。
type ForwardInfo struct {
	Name           string `json:"name"`
	Host           string `json:"host"`
	Type           string `json:"type"`
	LocalPort      int    `json:"local_port"`
	RemoteHost     string `json:"remote_host,omitempty"`
	RemotePort     int    `json:"remote_port,omitempty"`
	RemoteBindAddr string `json:"remote_bind_addr,omitempty"`
	AutoConnect    bool   `json:"auto_connect"`
}

// ForwardAddParams は forward.add リクエストのパラメータ。
type ForwardAddParams struct {
	Name           string `json:"name,omitempty"`
	Host           string `json:"host"`
	Type           string `json:"type"`
	LocalPort      int    `json:"local_port"`
	RemoteHost     string `json:"remote_host,omitempty"`
	RemotePort     int    `json:"remote_port,omitempty"`
	RemoteBindAddr string `json:"remote_bind_addr,omitempty"`
	AutoConnect    bool   `json:"auto_connect"`
}

// ForwardAddResult は forward.add リクエストの結果。
type ForwardAddResult struct {
	Name string `json:"name"`
}

// ForwardDeleteParams は forward.delete リクエストのパラメータ。
type ForwardDeleteParams struct {
	Name string `json:"name"`
}

// ForwardDeleteResult は forward.delete リクエストの結果。
type ForwardDeleteResult struct {
	OK bool `json:"ok"`
}

// ForwardStartParams は forward.start リクエストのパラメータ。
type ForwardStartParams struct {
	Name string `json:"name"`
}

// ForwardStartResult は forward.start リクエストの結果。
type ForwardStartResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ForwardStopParams は forward.stop リクエストのパラメータ。
type ForwardStopParams struct {
	Name string `json:"name"`
}

// ForwardStopResult は forward.stop リクエストの結果。
type ForwardStopResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ForwardStopAllResult は forward.stopAll リクエストの結果。
type ForwardStopAllResult struct {
	Stopped int `json:"stopped"`
}
