package core

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// ConnectionState は SSH 接続の状態を表す。
type ConnectionState int

const (
	Disconnected ConnectionState = iota
	Connecting
	Connected
	Reconnecting
	ConnectionError
)

func (s ConnectionState) String() string {
	switch s {
	case Disconnected:
		return "Disconnected"
	case Connecting:
		return "Connecting"
	case Connected:
		return "Connected"
	case Reconnecting:
		return "Reconnecting"
	case ConnectionError:
		return "Error"
	default:
		return fmt.Sprintf("ConnectionState(%d)", int(s))
	}
}

// SessionStatus はポートフォワーディングセッションの状態を表す。
type SessionStatus int

const (
	Stopped SessionStatus = iota
	Starting
	Active
	SessionReconnecting
	SessionError
)

func (s SessionStatus) String() string {
	switch s {
	case Stopped:
		return "Stopped"
	case Starting:
		return "Starting"
	case Active:
		return "Active"
	case SessionReconnecting:
		return "Reconnecting"
	case SessionError:
		return "Error"
	default:
		return fmt.Sprintf("SessionStatus(%d)", int(s))
	}
}

// ForwardType はポートフォワーディングの種別を表す。
type ForwardType int

const (
	Local ForwardType = iota
	Remote
	Dynamic
)

func (t ForwardType) String() string {
	switch t {
	case Local:
		return "local"
	case Remote:
		return "remote"
	case Dynamic:
		return "dynamic"
	default:
		return fmt.Sprintf("ForwardType(%d)", int(t))
	}
}

// ParseForwardType は文字列から ForwardType を解析する。
func ParseForwardType(s string) (ForwardType, error) {
	switch s {
	case "local":
		return Local, nil
	case "remote":
		return Remote, nil
	case "dynamic":
		return Dynamic, nil
	default:
		return 0, fmt.Errorf("unknown forward type: %q", s)
	}
}

// MarshalYAML は ForwardType を YAML 文字列としてシリアライズする。
func (t ForwardType) MarshalYAML() (interface{}, error) {
	return t.String(), nil
}

// UnmarshalYAML は YAML 文字列から ForwardType をデシリアライズする。
func (t *ForwardType) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := ParseForwardType(s)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// Duration は time.Duration のラッパーで、YAML シリアライズをサポートする。
type Duration struct {
	time.Duration
}

// MarshalYAML は Duration を文字列としてシリアライズする。
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// UnmarshalYAML は文字列から Duration をデシリアライズする。
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

// SSHHost は SSH config から読み込んだホスト情報と実行時の接続状態を保持する。
type SSHHost struct {
	Name               string
	HostName           string
	Port               int
	User               string
	IdentityFile       string
	ProxyJump          []string
	State              ConnectionState
	ActiveForwardCount int
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

// SSHEventType は SSH イベントの種別を表す。
type SSHEventType int

const (
	SSHEventConnected SSHEventType = iota
	SSHEventDisconnected
	SSHEventReconnecting
	SSHEventError
)

func (t SSHEventType) String() string {
	switch t {
	case SSHEventConnected:
		return "Connected"
	case SSHEventDisconnected:
		return "Disconnected"
	case SSHEventReconnecting:
		return "Reconnecting"
	case SSHEventError:
		return "Error"
	default:
		return fmt.Sprintf("SSHEventType(%d)", int(t))
	}
}

// SSHEvent は SSH 接続に関するイベント。
type SSHEvent struct {
	Type     SSHEventType
	HostName string
	Error    error
}

// ForwardEventType はポートフォワーディングイベントの種別を表す。
type ForwardEventType int

const (
	ForwardEventStarted ForwardEventType = iota
	ForwardEventStopped
	ForwardEventError
	ForwardEventMetricsUpdated
)

func (t ForwardEventType) String() string {
	switch t {
	case ForwardEventStarted:
		return "Started"
	case ForwardEventStopped:
		return "Stopped"
	case ForwardEventError:
		return "Error"
	case ForwardEventMetricsUpdated:
		return "MetricsUpdated"
	default:
		return fmt.Sprintf("ForwardEventType(%d)", int(t))
	}
}

// ForwardEvent はポートフォワーディングに関するイベント。
type ForwardEvent struct {
	Type     ForwardEventType
	RuleName string
	Session  *ForwardSession
	Error    error
}
