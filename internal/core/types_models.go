package core

import "time"

// SSHHost は SSH config から読み込んだホスト情報と実行時の接続状態を保持する。
type SSHHost struct {
	Name                  string
	HostName              string
	Port                  int
	User                  string
	IdentityFile          string
	ProxyJump             []string
	ProxyCommand          string
	StrictHostKeyChecking string
	State                 ConnectionState
	ActiveForwardCount    int
}

// ForwardRule はポートフォワーディングのルール定義。
type ForwardRule struct {
	Name        string      `yaml:"name"`
	Host        string      `yaml:"host"`
	Type        ForwardType `yaml:"type"`
	LocalPort   int         `yaml:"local_port"`
	RemoteHost  string      `yaml:"remote_host,omitempty"`
	RemotePort  int         `yaml:"remote_port,omitempty"`
	AutoConnect bool        `yaml:"auto_connect"`
}

// ForwardSession は実行中のポートフォワーディングセッションの状態とメトリクスを保持する。
type ForwardSession struct {
	ID             string
	Rule           ForwardRule
	Status         SessionStatus
	ConnectedAt    time.Time
	BytesSent      int64
	BytesReceived  int64
	ReconnectCount int
	LastError      string
}

// ForwardRestoreResult はフォワード復元の結果を表す。
type ForwardRestoreResult struct {
	RuleName string
	OK       bool
	Error    string
}

// Config はアプリケーション設定。
type Config struct {
	SSHConfigPath string                `yaml:"ssh_config_path"`
	Reconnect     ReconnectConfig       `yaml:"reconnect"`
	Hosts         map[string]HostConfig `yaml:"hosts,omitempty"`
	Session       SessionConfig         `yaml:"session"`
	Log           LogConfig             `yaml:"log"`
	Forwards      []ForwardRule         `yaml:"forwards"`
	Language      string                `yaml:"language"`
	TUI           TUIConfig             `yaml:"tui"`
}

// ReconnectConfig は自動再接続の設定。
type ReconnectConfig struct {
	Enabled           bool     `yaml:"enabled"`
	MaxRetries        int      `yaml:"max_retries"`
	InitialDelay      Duration `yaml:"initial_delay"`
	MaxDelay          Duration `yaml:"max_delay"`
	KeepAliveInterval Duration `yaml:"keepalive_interval"`
}

// ReconnectOverride はホスト別の再接続設定オーバーライド。
// 指定されたフィールドのみグローバル設定を上書きする。
type ReconnectOverride struct {
	Enabled      *bool     `yaml:"enabled,omitempty"`
	MaxRetries   *int      `yaml:"max_retries,omitempty"`
	InitialDelay *Duration `yaml:"initial_delay,omitempty"`
	MaxDelay     *Duration `yaml:"max_delay,omitempty"`
}

// HostConfig はホスト別のオーバーライド設定。
type HostConfig struct {
	Reconnect *ReconnectOverride `yaml:"reconnect,omitempty"`
}

// SessionConfig はセッション復元の設定。
type SessionConfig struct {
	AutoRestore bool `yaml:"auto_restore"`
}

// LogConfig はログの設定。
type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// TUIConfig は TUI の設定。
type TUIConfig struct {
	Theme ThemeConfig `yaml:"theme"`
}

// ThemeConfig はテーマの設定。
type ThemeConfig struct {
	Base   string `yaml:"base"`
	Accent string `yaml:"accent"`
}

// State はアプリケーション終了時のセッション状態を保持する。
type State struct {
	LastUpdated    time.Time     `yaml:"last_updated"`
	ActiveForwards []ForwardRule `yaml:"active_forwards"`
	SelectedHost   string        `yaml:"selected_host"`
}

// DefaultConfig はデフォルト設定を返す。
func DefaultConfig() Config {
	return Config{
		SSHConfigPath: "~/.ssh/config",
		Reconnect: ReconnectConfig{
			Enabled:           true,
			MaxRetries:        10,
			InitialDelay:      Duration{Duration: 1 * time.Second},
			MaxDelay:          Duration{Duration: 60 * time.Second},
			KeepAliveInterval: Duration{Duration: 30 * time.Second},
		},
		Session: SessionConfig{
			AutoRestore: true,
		},
		Log: LogConfig{
			Level: "info",
			File:  "~/.config/moleport/moleport.log",
		},
	}
}
