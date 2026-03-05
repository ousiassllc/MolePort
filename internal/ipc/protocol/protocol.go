package protocol

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

// Scanner バッファサイズ定数。
const (
	ScannerInitBuf = 64 * 1024   // 64KB
	ScannerMaxBuf  = 1024 * 1024 // 1MB
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
	CredentialTimeout    = 1008
	CredentialCancelled  = 1009
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
