package protocol

// --- 設定管理 ---

// ConfigGetParams は config.get リクエストのパラメータ。
type ConfigGetParams struct{}

// ConfigGetResult は config.get リクエストの結果。
type ConfigGetResult struct {
	SSHConfigPath string         `json:"ssh_config_path"`
	Reconnect     ReconnectInfo  `json:"reconnect"`
	Session       SessionCfgInfo `json:"session"`
	Log           LogInfo        `json:"log"`
}

// ReconnectInfo は再接続設定の情報を表す。
type ReconnectInfo struct {
	Enabled      bool   `json:"enabled"`
	MaxRetries   int    `json:"max_retries"`
	InitialDelay string `json:"initial_delay"`
	MaxDelay     string `json:"max_delay"`
}

// SessionCfgInfo はセッション設定の情報を表す。
type SessionCfgInfo struct {
	AutoRestore bool `json:"auto_restore"`
}

// LogInfo はログ設定の情報を表す。
type LogInfo struct {
	Level string `json:"level"`
	File  string `json:"file"`
}

// ConfigUpdateParams は config.update リクエストのパラメータ（部分更新）。
// 各フィールドはポインタ型で、nil なら変更なしを意味する。
type ConfigUpdateParams struct {
	SSHConfigPath *string               `json:"ssh_config_path,omitempty"`
	Reconnect     *ReconnectUpdateInfo  `json:"reconnect,omitempty"`
	Session       *SessionCfgUpdateInfo `json:"session,omitempty"`
	Log           *LogUpdateInfo        `json:"log,omitempty"`
}

// ReconnectUpdateInfo は再接続設定の部分更新パラメータ。
// nil フィールドは変更なしを意味する。
type ReconnectUpdateInfo struct {
	Enabled      *bool   `json:"enabled,omitempty"`
	MaxRetries   *int    `json:"max_retries,omitempty"`
	InitialDelay *string `json:"initial_delay,omitempty"`
	MaxDelay     *string `json:"max_delay,omitempty"`
}

// SessionCfgUpdateInfo はセッション設定の部分更新パラメータ。
type SessionCfgUpdateInfo struct {
	AutoRestore *bool `json:"auto_restore,omitempty"`
}

// LogUpdateInfo はログ設定の部分更新パラメータ。
type LogUpdateInfo struct {
	Level *string `json:"level,omitempty"`
	File  *string `json:"file,omitempty"`
}

// ConfigUpdateResult は config.update リクエストの結果。
type ConfigUpdateResult struct {
	OK bool `json:"ok"`
}
