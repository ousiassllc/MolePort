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
	PendingAuth
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
	case PendingAuth:
		return "PendingAuth"
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
func (t ForwardType) MarshalYAML() (any, error) {
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
func (d Duration) MarshalYAML() (any, error) {
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
