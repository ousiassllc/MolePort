package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRequest_JSONRoundtrip(t *testing.T) {
	id := 1
	params := json.RawMessage(`{"host":"example"}`)
	req := Request{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
		Method:  "ssh.connect",
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal Request: %v", err)
	}

	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal Request: %v", err)
	}

	if got.JSONRPC != req.JSONRPC {
		t.Errorf("JSONRPC = %q, want %q", got.JSONRPC, req.JSONRPC)
	}
	if got.ID == nil || *got.ID != *req.ID {
		t.Errorf("ID = %v, want %v", got.ID, req.ID)
	}
	if got.Method != req.Method {
		t.Errorf("Method = %q, want %q", got.Method, req.Method)
	}
	if string(got.Params) != string(req.Params) {
		t.Errorf("Params = %s, want %s", got.Params, req.Params)
	}
}

func TestResponse_JSONRoundtrip(t *testing.T) {
	id := 42
	result := json.RawMessage(`{"host":"example","status":"connected"}`)
	resp := Response{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal Response: %v", err)
	}

	var got Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal Response: %v", err)
	}

	if got.JSONRPC != resp.JSONRPC {
		t.Errorf("JSONRPC = %q, want %q", got.JSONRPC, resp.JSONRPC)
	}
	if got.ID == nil || *got.ID != *resp.ID {
		t.Errorf("ID = %v, want %v", got.ID, resp.ID)
	}
	if string(got.Result) != string(resp.Result) {
		t.Errorf("Result = %s, want %s", got.Result, resp.Result)
	}
	if got.Error != nil {
		t.Errorf("Error = %v, want nil", got.Error)
	}
}

func TestResponse_NilID_MarshalAsNull(t *testing.T) {
	resp := NewErrorResponse(nil, ParseError, "parse error")
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal Response: %v", err)
	}
	if !strings.Contains(string(data), `"id":null`) {
		t.Errorf("Response with nil ID should serialize as null, got: %s", data)
	}
}

func TestNotification_JSONRoundtrip(t *testing.T) {
	params := json.RawMessage(`{"type":"connected","host":"prod"}`)
	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  "event.ssh",
		Params:  params,
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Marshal Notification: %v", err)
	}

	var got Notification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal Notification: %v", err)
	}

	if got.JSONRPC != notif.JSONRPC {
		t.Errorf("JSONRPC = %q, want %q", got.JSONRPC, notif.JSONRPC)
	}
	if got.Method != notif.Method {
		t.Errorf("Method = %q, want %q", got.Method, notif.Method)
	}
	if string(got.Params) != string(notif.Params) {
		t.Errorf("Params = %s, want %s", got.Params, notif.Params)
	}
}

func TestNotification_OmitsIDField(t *testing.T) {
	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  "event.ssh",
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Marshal Notification: %v", err)
	}

	if strings.Contains(string(data), `"id"`) {
		t.Errorf("Notification JSON should not contain 'id' field, got: %s", data)
	}
}

func TestRequest_NilID_OmitsIDField(t *testing.T) {
	req := Request{
		JSONRPC: JSONRPCVersion,
		Method:  "event.ssh",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal Request: %v", err)
	}

	if strings.Contains(string(data), `"id"`) {
		t.Errorf("Request with nil ID should not contain 'id' field, got: %s", data)
	}
}

func TestRPCError_Error(t *testing.T) {
	e := &RPCError{Code: MethodNotFound, Message: "method not found"}
	got := e.Error()
	if !strings.Contains(got, "-32601") {
		t.Errorf("Error() should contain code -32601, got: %q", got)
	}
	if !strings.Contains(got, "method not found") {
		t.Errorf("Error() should contain message, got: %q", got)
	}
}

func TestNewResponse(t *testing.T) {
	id := 1
	result := SSHConnectResult{Host: "prod", Status: "connected"}
	resp, err := NewResponse(&id, result)
	if err != nil {
		t.Fatalf("NewResponse: %v", err)
	}

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, JSONRPCVersion)
	}
	if resp.ID == nil || *resp.ID != 1 {
		t.Errorf("ID = %v, want 1", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Error = %v, want nil", resp.Error)
	}

	var got SSHConnectResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}
	if got.Host != "prod" || got.Status != "connected" {
		t.Errorf("Result = %+v, want {Host:prod Status:connected}", got)
	}
}

func TestNewErrorResponse(t *testing.T) {
	id := 5
	resp := NewErrorResponse(&id, InternalError, "something went wrong")

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, JSONRPCVersion)
	}
	if resp.ID == nil || *resp.ID != 5 {
		t.Errorf("ID = %v, want 5", resp.ID)
	}
	if resp.Result != nil {
		t.Errorf("Result = %s, want nil", resp.Result)
	}
	if resp.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if resp.Error.Code != InternalError {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, InternalError)
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("Error.Message = %q, want %q", resp.Error.Message, "something went wrong")
	}
}

func TestErrorCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"ParseError", ParseError, -32700},
		{"InvalidRequest", InvalidRequest, -32600},
		{"MethodNotFound", MethodNotFound, -32601},
		{"InvalidParams", InvalidParams, -32602},
		{"InternalError", InternalError, -32603},
		{"HostNotFound", HostNotFound, 1001},
		{"AlreadyConnected", AlreadyConnected, 1002},
		{"NotConnected", NotConnected, 1003},
		{"RuleNotFound", RuleNotFound, 1004},
		{"RuleAlreadyExists", RuleAlreadyExists, 1005},
		{"PortConflict", PortConflict, 1006},
		{"AuthenticationFailed", AuthenticationFailed, 1007},
	}
	for _, tt := range tests {
		if tt.code != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
		}
	}
}
