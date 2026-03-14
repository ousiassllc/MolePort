package protocol

// --- セッション情報 ---

// SessionListParams は session.list リクエストのパラメータ。
type SessionListParams struct{}

// SessionListResult は session.list リクエストの結果。
type SessionListResult struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo はポートフォワーディングセッションの情報を表す。
type SessionInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Host           string `json:"host"`
	Type           string `json:"type"`
	LocalPort      int    `json:"local_port"`
	RemoteHost     string `json:"remote_host,omitempty"`
	RemotePort     int    `json:"remote_port,omitempty"`
	RemoteBindAddr string `json:"remote_bind_addr,omitempty"`
	Status         string `json:"status"`
	ConnectedAt    string `json:"connected_at,omitempty"`
	BytesSent      int64  `json:"bytes_sent"`
	BytesReceived  int64  `json:"bytes_received"`
	ReconnectCount int    `json:"reconnect_count"`
	LastError      string `json:"last_error,omitempty"`
}

// SessionGetParams は session.get リクエストのパラメータ。
type SessionGetParams struct {
	Name string `json:"name"`
}

// SessionGetResult は session.get リクエストの結果（SessionInfo のエイリアス）。
type SessionGetResult = SessionInfo
