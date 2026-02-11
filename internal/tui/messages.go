package tui

import (
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
)

// FocusPane はフォーカス中のペインを示す。
type FocusPane int

const (
	PaneForwards FocusPane = iota
	PaneSetup
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

// ForwardAddRequestMsg はセットアップウィザード完了時に発行される。
type ForwardAddRequestMsg struct {
	Host        string
	Type        core.ForwardType
	LocalPort   int
	RemoteHost  string
	RemotePort  int
	Name        string
	AutoConnect bool
}

// LogOutputMsg はログ出力テキスト。
type LogOutputMsg struct {
	Text string
}

// QuitRequestMsg はアプリケーション終了を要求する。
type QuitRequestMsg struct{}

// IPCNotificationMsg は IPC から受信した通知メッセージ。
type IPCNotificationMsg struct {
	Notification *ipc.Notification
}

// IPCDisconnectedMsg は IPC 接続断を通知する。
type IPCDisconnectedMsg struct{}

// CredentialRequestMsg はデーモンからのクレデンシャル要求を TUI に伝える。
// ResponseCh に応答を書き込むとクレデンシャルがデーモンに返される。
type CredentialRequestMsg struct {
	Request    ipc.CredentialRequestNotification
	ResponseCh chan<- *ipc.CredentialResponseParams
}

// CredentialSubmitMsg はパスワード入力完了時に発行される内部メッセージ。
type CredentialSubmitMsg struct {
	Value     string
	Cancelled bool
}
