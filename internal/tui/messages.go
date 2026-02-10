package tui

import "github.com/ousiassllc/moleport/internal/core"

// FocusPane はフォーカス中のペインを示す。
type FocusPane int

const (
	PaneHostList FocusPane = iota
	PaneForward
	PaneCommand
)

// HostSelectedMsg はホスト一覧でカーソルが移動したときに発行される。
type HostSelectedMsg struct {
	Host core.SSHHost
}

// HostsLoadedMsg はホスト一覧の初期読み込み完了時に発行される。
type HostsLoadedMsg struct {
	Hosts []core.SSHHost
	Err   error
}

// HostsReloadedMsg はホスト一覧の再読み込み完了時に発行される。
type HostsReloadedMsg struct {
	Hosts []core.SSHHost
	Err   error
}

// ForwardToggleMsg はフォワーディングの開始/停止を要求する。
type ForwardToggleMsg struct {
	RuleName string
}

// ForwardDeleteRequestMsg はフォワーディングルールの削除確認を要求する。
type ForwardDeleteRequestMsg struct {
	RuleName string
}

// ForwardDeleteConfirmedMsg はフォワーディングルールの削除を確定する。
type ForwardDeleteConfirmedMsg struct {
	RuleName string
}

// ForwardUpdatedMsg はフォワーディングイベントの通知。
type ForwardUpdatedMsg struct {
	Event core.ForwardEvent
}

// SSHEventMsg は SSH イベントの通知。
type SSHEventMsg struct {
	Event core.SSHEvent
}

// MetricsTickMsg はメトリクス更新のティック。
type MetricsTickMsg struct{}

// CommandExecuteMsg はコマンド実行を要求する。
type CommandExecuteMsg struct {
	Command string
	Values  map[string]string
}

// CommandOutputMsg はコマンド実行結果のテキスト出力。
type CommandOutputMsg struct {
	Text string
}

// QuitRequestMsg はアプリケーション終了を要求する。
type QuitRequestMsg struct{}

// SessionRestoredMsg はセッション復元の完了通知。
type SessionRestoredMsg struct {
	Err error
}
