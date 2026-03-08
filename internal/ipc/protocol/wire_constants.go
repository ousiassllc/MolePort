package protocol

// IPC ワイヤーフォーマット上の接続状態文字列定数。
// core.ConnectionState.String() は大文字始まり ("Connected" 等) を返すが、
// IPC ワイヤーでは小文字スネークケースを使用する。
const (
	StateConnected    = "connected"
	StateConnecting   = "connecting"
	StateDisconnected = "disconnected"
	StateReconnecting = "reconnecting"
	StatePendingAuth  = "pending_auth"
	StateError        = "error"
)

// IPC ワイヤーフォーマット上のセッション状態文字列定数。
const (
	SessionActive       = "active"
	SessionStarting     = "starting"
	SessionStopped      = "stopped"
	SessionReconnecting = "reconnecting"
	SessionError        = "error"
)

// IPC ワイヤーフォーマット上のフォワード種別文字列定数。
// core.ForwardType.String() と同じ値だが、ワイヤー仕様として明示的に定義する。
const (
	ForwardTypeLocal   = "local"
	ForwardTypeRemote  = "remote"
	ForwardTypeDynamic = "dynamic"
)

// RPC メソッド名定数。
const (
	MethodEventsSubscribe    = "events.subscribe"
	MethodEventsUnsubscribe  = "events.unsubscribe"
	MethodCredentialRequest  = "credential.request"  //nolint:gosec // RPC method name, not a credential
	MethodCredentialResponse = "credential.response" //nolint:gosec // RPC method name, not a credential
)
