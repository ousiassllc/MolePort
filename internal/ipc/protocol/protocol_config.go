package protocol

// --- 設定管理 ---

// ConfigGetParams は config.get リクエストのパラメータ。
type ConfigGetParams struct{}

// ConfigGetResult は config.get リクエストの結果。
type ConfigGetResult struct {
	SSHConfigPath string                    `json:"ssh_config_path"`
	Reconnect     ReconnectInfo             `json:"reconnect"`
	Hosts         map[string]HostConfigInfo `json:"hosts,omitempty"`
	Session       SessionCfgInfo            `json:"session"`
	Log           LogInfo                   `json:"log"`
	Language      string                    `json:"language"`
	UpdateCheck   UpdateCheckInfo           `json:"update_check"`
	TUI           TUIInfo                   `json:"tui"`
}

// UpdateCheckInfo はアップデートチェック設定の情報を表す。
type UpdateCheckInfo struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
}

// HostConfigInfo はホスト別設定の情報を表す。
type HostConfigInfo struct {
	Reconnect *ReconnectOverrideInfo `json:"reconnect,omitempty"`
}

// ReconnectOverrideInfo はホスト別の再接続設定オーバーライド情報を表す。
type ReconnectOverrideInfo struct {
	Enabled      *bool   `json:"enabled,omitempty"`
	MaxRetries   *int    `json:"max_retries,omitempty"`
	InitialDelay *string `json:"initial_delay,omitempty"`
	MaxDelay     *string `json:"max_delay,omitempty"`
}

// ReconnectInfo は再接続設定の情報を表す。
type ReconnectInfo struct {
	Enabled           bool   `json:"enabled"`
	MaxRetries        int    `json:"max_retries"`
	InitialDelay      string `json:"initial_delay"`
	MaxDelay          string `json:"max_delay"`
	KeepAliveInterval string `json:"keepalive_interval"`
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

// TUIInfo は TUI 設定の情報を表す。
type TUIInfo struct {
	Theme ThemeInfo `json:"theme"`
}

// ThemeInfo はテーマ設定の情報を表す。
type ThemeInfo struct {
	Base   string `json:"base"`
	Accent string `json:"accent"`
}

// ConfigUpdateParams は config.update リクエストのパラメータ（部分更新）。
// 各フィールドはポインタ型で、nil なら変更なしを意味する。
type ConfigUpdateParams struct {
	SSHConfigPath *string                          `json:"ssh_config_path,omitempty"`
	Reconnect     *ReconnectUpdateInfo             `json:"reconnect,omitempty"`
	Hosts         map[string]*HostConfigUpdateInfo `json:"hosts,omitempty"`
	Session       *SessionCfgUpdateInfo            `json:"session,omitempty"`
	Log           *LogUpdateInfo                   `json:"log,omitempty"`
	Language      *string                          `json:"language,omitempty"`
	UpdateCheck   *UpdateCheckUpdateInfo           `json:"update_check,omitempty"`
	TUI           *TUIUpdateInfo                   `json:"tui,omitempty"`
}

// UpdateCheckUpdateInfo はアップデートチェック設定の部分更新パラメータ。
type UpdateCheckUpdateInfo struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	Interval *string `json:"interval,omitempty"`
}

// HostConfigUpdateInfo はホスト別設定の部分更新パラメータ。
// ReconnectUpdateInfo を共有型として再利用する。KeepAliveInterval はホスト別では無視される。
type HostConfigUpdateInfo struct {
	Reconnect *ReconnectUpdateInfo `json:"reconnect,omitempty"`
}

// ReconnectUpdateInfo は再接続設定の部分更新パラメータ。
// nil フィールドは変更なしを意味する。
type ReconnectUpdateInfo struct {
	Enabled           *bool   `json:"enabled,omitempty"`
	MaxRetries        *int    `json:"max_retries,omitempty"`
	InitialDelay      *string `json:"initial_delay,omitempty"`
	MaxDelay          *string `json:"max_delay,omitempty"`
	KeepAliveInterval *string `json:"keepalive_interval,omitempty"`
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

// TUIUpdateInfo は TUI 設定の部分更新パラメータ。
type TUIUpdateInfo struct {
	Theme *ThemeUpdateInfo `json:"theme,omitempty"`
}

// ThemeUpdateInfo はテーマ設定の部分更新パラメータ。
type ThemeUpdateInfo struct {
	Base   *string `json:"base,omitempty"`
	Accent *string `json:"accent,omitempty"`
}

// ConfigUpdateResult は config.update リクエストの結果。
type ConfigUpdateResult struct {
	OK bool `json:"ok"`
}
