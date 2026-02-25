package protocol

// --- ホスト管理 ---

// HostListParams は host.list リクエストのパラメータ。
type HostListParams struct{}

// HostListResult は host.list リクエストの結果。
type HostListResult struct {
	Hosts []HostInfo `json:"hosts"`
}

// HostInfo は SSH ホストの情報を表す。
type HostInfo struct {
	Name               string `json:"name"`
	HostName           string `json:"hostname"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	State              string `json:"state"`
	ActiveForwardCount int    `json:"active_forward_count"`
}

// HostReloadParams は host.reload リクエストのパラメータ。
type HostReloadParams struct{}

// HostReloadResult は host.reload リクエストの結果。
type HostReloadResult struct {
	Total   int      `json:"total"`
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}
