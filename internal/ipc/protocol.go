package ipc

import (
	"encoding/json"
	"fmt"
)

// JSONRPCVersion は JSON-RPC プロトコルバージョンを表す。
const JSONRPCVersion = "2.0"

// 標準 JSON-RPC エラーコード。
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// アプリケーション固有のエラーコード。
const (
	HostNotFound         = 1001
	AlreadyConnected     = 1002
	NotConnected         = 1003
	RuleNotFound         = 1004
	RuleAlreadyExists    = 1005
	PortConflict         = 1006
	AuthenticationFailed = 1007
)

// Request は JSON-RPC 2.0 リクエストを表す。
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response は JSON-RPC 2.0 レスポンスを表す。
// ID は *int を使用する。JSON-RPC 2.0 仕様では、パース不能なリクエストへのレスポンスで
// "id": null を返す必要があるため。
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Notification は JSON-RPC 2.0 通知（ID なし）を表す。
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCError は JSON-RPC 2.0 エラーオブジェクトを表す。
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error は RPCError を文字列として返す。
func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error: code=%d, message=%s", e.Code, e.Message)
}

// NewResponse は result を JSON にマーシャルして Response を生成する。
func NewResponse(id *int, result any) (Response, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return Response{}, fmt.Errorf("marshal result: %w", err)
	}
	return Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  data,
	}, nil
}

// NewErrorResponse はエラーコードとメッセージから Response を生成する。
func NewErrorResponse(id *int, code int, message string) Response {
	return Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
}

// intPtr は int のポインタを返すヘルパー。
func intPtr(v int) *int { return &v }

// --- ホスト管理 ---

// HostListParams は host.list リクエストのパラメータ。
type HostListParams struct{}

// HostListResult は host.list リクエストの結果。
type HostListResult struct {
	Hosts []HostInfo `json:"hosts"`
}

// HostInfo は SSH ホストの情報を表す。
type HostInfo struct {
	Name               string `json:"name"`
	HostName           string `json:"hostname"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	State              string `json:"state"`
	ActiveForwardCount int    `json:"active_forward_count"`
}

// HostReloadParams は host.reload リクエストのパラメータ。
type HostReloadParams struct{}

// HostReloadResult は host.reload リクエストの結果。
type HostReloadResult struct {
	Total   int      `json:"total"`
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}

// --- SSH 接続管理 ---

// SSHConnectParams は ssh.connect リクエストのパラメータ。
type SSHConnectParams struct {
	Host string `json:"host"`
}

// SSHConnectResult は ssh.connect リクエストの結果。
type SSHConnectResult struct {
	Host   string `json:"host"`
	Status string `json:"status"`
}

// SSHDisconnectParams は ssh.disconnect リクエストのパラメータ。
type SSHDisconnectParams struct {
	Host string `json:"host"`
}

// SSHDisconnectResult は ssh.disconnect リクエストの結果。
type SSHDisconnectResult struct {
	Host   string `json:"host"`
	Status string `json:"status"`
}

// --- ポートフォワーディング管理 ---

// ForwardListParams は forward.list リクエストのパラメータ。
type ForwardListParams struct {
	Host string `json:"host,omitempty"`
}

// ForwardListResult は forward.list リクエストの結果。
type ForwardListResult struct {
	Forwards []ForwardInfo `json:"forwards"`
}

// ForwardInfo はポートフォワーディングルールの情報を表す。
type ForwardInfo struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Type        string `json:"type"`
	LocalPort   int    `json:"local_port"`
	RemoteHost  string `json:"remote_host,omitempty"`
	RemotePort  int    `json:"remote_port,omitempty"`
	AutoConnect bool   `json:"auto_connect"`
}

// ForwardAddParams は forward.add リクエストのパラメータ。
type ForwardAddParams struct {
	Name        string `json:"name,omitempty"`
	Host        string `json:"host"`
	Type        string `json:"type"`
	LocalPort   int    `json:"local_port"`
	RemoteHost  string `json:"remote_host,omitempty"`
	RemotePort  int    `json:"remote_port,omitempty"`
	AutoConnect bool   `json:"auto_connect"`
}

// ForwardAddResult は forward.add リクエストの結果。
type ForwardAddResult struct {
	Name string `json:"name"`
}

// ForwardDeleteParams は forward.delete リクエストのパラメータ。
type ForwardDeleteParams struct {
	Name string `json:"name"`
}

// ForwardDeleteResult は forward.delete リクエストの結果。
type ForwardDeleteResult struct {
	OK bool `json:"ok"`
}

// ForwardStartParams は forward.start リクエストのパラメータ。
type ForwardStartParams struct {
	Name string `json:"name"`
}

// ForwardStartResult は forward.start リクエストの結果。
type ForwardStartResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ForwardStopParams は forward.stop リクエストのパラメータ。
type ForwardStopParams struct {
	Name string `json:"name"`
}

// ForwardStopResult は forward.stop リクエストの結果。
type ForwardStopResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

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

// --- デーモン管理 ---

// DaemonStatusParams は daemon.status リクエストのパラメータ。
type DaemonStatusParams struct{}

// DaemonStatusResult は daemon.status リクエストの結果。
type DaemonStatusResult struct {
	PID                  int    `json:"pid"`
	StartedAt            string `json:"started_at"`
	Uptime               string `json:"uptime"`
	ConnectedClients     int    `json:"connected_clients"`
	ActiveSSHConnections int    `json:"active_ssh_connections"`
	ActiveForwards       int    `json:"active_forwards"`
}

// DaemonShutdownParams は daemon.shutdown リクエストのパラメータ。
type DaemonShutdownParams struct{}

// DaemonShutdownResult は daemon.shutdown リクエストの結果。
type DaemonShutdownResult struct {
	OK bool `json:"ok"`
}

// --- イベントサブスクリプション ---

// EventsSubscribeParams は events.subscribe リクエストのパラメータ。
type EventsSubscribeParams struct {
	Types []string `json:"types"`
}

// EventsSubscribeResult は events.subscribe リクエストの結果。
type EventsSubscribeResult struct {
	SubscriptionID string `json:"subscription_id"`
}

// EventsUnsubscribeParams は events.unsubscribe リクエストのパラメータ。
type EventsUnsubscribeParams struct {
	SubscriptionID string `json:"subscription_id"`
}

// EventsUnsubscribeResult は events.unsubscribe リクエストの結果。
type EventsUnsubscribeResult struct {
	OK bool `json:"ok"`
}

// --- イベント通知 ---

// SSHEventNotification は SSH イベント通知を表す。
type SSHEventNotification struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Error string `json:"error,omitempty"`
}

// ForwardEventNotification はポートフォワーディングイベント通知を表す。
type ForwardEventNotification struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Host  string `json:"host"`
	Error string `json:"error,omitempty"`
}

// MetricsEventNotification はメトリクスイベント通知を表す。
type MetricsEventNotification struct {
	Sessions []SessionMetrics `json:"sessions"`
}

// SessionMetrics はセッションのメトリクス情報を表す。
type SessionMetrics struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	BytesSent     int64  `json:"bytes_sent"`
	BytesReceived int64  `json:"bytes_received"`
	Uptime        string `json:"uptime"`
}
