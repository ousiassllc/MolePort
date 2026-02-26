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

// Config はアプリケーション設定。
type Config struct {
	SSHConfigPath string          `yaml:"ssh_config_path"`
	Reconnect     ReconnectConfig `yaml:"reconnect"`
	Session       SessionConfig   `yaml:"session"`
	Log           LogConfig       `yaml:"log"`
	Forwards      []ForwardRule   `yaml:"forwards"`
}

// ReconnectConfig は自動再接続の設定。
type ReconnectConfig struct {
	Enabled      bool     `yaml:"enabled"`
	MaxRetries   int      `yaml:"max_retries"`
	InitialDelay Duration `yaml:"initial_delay"`
	MaxDelay     Duration `yaml:"max_delay"`
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
			Enabled:      true,
			MaxRetries:   10,
			InitialDelay: Duration{Duration: 1 * time.Second},
			MaxDelay:     Duration{Duration: 60 * time.Second},
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
