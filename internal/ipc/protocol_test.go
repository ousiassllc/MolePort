package ipc

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

func TestHostListResult_JSONRoundtrip(t *testing.T) {
	original := HostListResult{
		Hosts: []HostInfo{
			{
				Name:               "prod",
				HostName:           "192.168.1.1",
				Port:               22,
				User:               "admin",
				State:              "connected",
				ActiveForwardCount: 3,
			},
			{
				Name:               "staging",
				HostName:           "192.168.1.2",
				Port:               2222,
				User:               "deploy",
				State:              "disconnected",
				ActiveForwardCount: 0,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal HostListResult: %v", err)
	}

	var got HostListResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal HostListResult: %v", err)
	}

	if len(got.Hosts) != 2 {
		t.Fatalf("len(Hosts) = %d, want 2", len(got.Hosts))
	}
	if got.Hosts[0].Name != "prod" {
		t.Errorf("Hosts[0].Name = %q, want %q", got.Hosts[0].Name, "prod")
	}
	if got.Hosts[0].ActiveForwardCount != 3 {
		t.Errorf("Hosts[0].ActiveForwardCount = %d, want 3", got.Hosts[0].ActiveForwardCount)
	}
	if got.Hosts[1].State != "disconnected" {
		t.Errorf("Hosts[1].State = %q, want %q", got.Hosts[1].State, "disconnected")
	}
}

func TestForwardInfo_JSONRoundtrip_WithOptionalFields(t *testing.T) {
	original := ForwardInfo{
		Name:        "web",
		Host:        "prod",
		Type:        "local",
		LocalPort:   8080,
		RemoteHost:  "localhost",
		RemotePort:  80,
		AutoConnect: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal ForwardInfo: %v", err)
	}

	var got ForwardInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ForwardInfo: %v", err)
	}

	if got != original {
		t.Errorf("ForwardInfo roundtrip: got %+v, want %+v", got, original)
	}
}

func TestForwardInfo_JSONRoundtrip_WithoutOptionalFields(t *testing.T) {
	original := ForwardInfo{
		Name:        "proxy",
		Host:        "staging",
		Type:        "dynamic",
		LocalPort:   1080,
		AutoConnect: false,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal ForwardInfo: %v", err)
	}

	// dynamic の場合、RemoteHost/RemotePort は omitempty で省略される
	if strings.Contains(string(data), `"remote_host"`) {
		t.Errorf("ForwardInfo JSON should omit remote_host when empty, got: %s", data)
	}
	if strings.Contains(string(data), `"remote_port"`) {
		t.Errorf("ForwardInfo JSON should omit remote_port when zero, got: %s", data)
	}

	var got ForwardInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ForwardInfo: %v", err)
	}

	if got != original {
		t.Errorf("ForwardInfo roundtrip: got %+v, want %+v", got, original)
	}
}

func TestSessionInfo_JSONRoundtrip(t *testing.T) {
	original := SessionInfo{
		ID:             "prod-local-8080",
		Name:           "web",
		Host:           "prod",
		Type:           "local",
		LocalPort:      8080,
		RemoteHost:     "localhost",
		RemotePort:     80,
		Status:         "active",
		ConnectedAt:    "2026-02-11T15:30:00+09:00",
		BytesSent:      1024,
		BytesReceived:  2048,
		ReconnectCount: 1,
		LastError:      "connection reset",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal SessionInfo: %v", err)
	}

	var got SessionInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal SessionInfo: %v", err)
	}

	if got != original {
		t.Errorf("SessionInfo roundtrip: got %+v, want %+v", got, original)
	}
}

func TestSessionInfo_JSONRoundtrip_OptionalFieldsOmitted(t *testing.T) {
	original := SessionInfo{
		ID:        "prod-local-8080",
		Name:      "web",
		Host:      "prod",
		Type:      "local",
		LocalPort: 8080,
		Status:    "stopped",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal SessionInfo: %v", err)
	}

	if strings.Contains(string(data), `"connected_at"`) {
		t.Errorf("SessionInfo JSON should omit connected_at when empty, got: %s", data)
	}
	if strings.Contains(string(data), `"last_error"`) {
		t.Errorf("SessionInfo JSON should omit last_error when empty, got: %s", data)
	}
}

func TestConfigUpdateParams_PointerFields(t *testing.T) {
	path := "/custom/ssh/config"
	params := ConfigUpdateParams{
		SSHConfigPath: &path,
		Reconnect:     nil,
		Session:       nil,
		Log:           nil,
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal ConfigUpdateParams: %v", err)
	}

	// nil ポインタフィールドは omitempty で省略される
	if strings.Contains(string(data), `"reconnect"`) {
		t.Errorf("ConfigUpdateParams JSON should omit nil reconnect, got: %s", data)
	}
	if strings.Contains(string(data), `"session"`) {
		t.Errorf("ConfigUpdateParams JSON should omit nil session, got: %s", data)
	}
	if strings.Contains(string(data), `"log"`) {
		t.Errorf("ConfigUpdateParams JSON should omit nil log, got: %s", data)
	}

	var got ConfigUpdateParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ConfigUpdateParams: %v", err)
	}

	if got.SSHConfigPath == nil || *got.SSHConfigPath != path {
		t.Errorf("SSHConfigPath = %v, want %q", got.SSHConfigPath, path)
	}
	if got.Reconnect != nil {
		t.Errorf("Reconnect = %v, want nil", got.Reconnect)
	}
}

func TestConfigUpdateParams_AllFields(t *testing.T) {
	path := "/custom/ssh/config"
	enabled := true
	maxRetries := 5
	initialDelay := "2s"
	maxDelay := "30s"
	autoRestore := false
	level := "debug"
	file := "/tmp/test.log"

	params := ConfigUpdateParams{
		SSHConfigPath: &path,
		Reconnect: &ReconnectUpdateInfo{
			Enabled: &enabled, MaxRetries: &maxRetries,
			InitialDelay: &initialDelay, MaxDelay: &maxDelay,
		},
		Session: &SessionCfgUpdateInfo{AutoRestore: &autoRestore},
		Log:     &LogUpdateInfo{Level: &level, File: &file},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal ConfigUpdateParams: %v", err)
	}

	var got ConfigUpdateParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ConfigUpdateParams: %v", err)
	}

	if got.Reconnect == nil || got.Reconnect.MaxRetries == nil || *got.Reconnect.MaxRetries != 5 {
		t.Errorf("Reconnect.MaxRetries = %v, want 5", got.Reconnect)
	}
	if got.Session == nil || got.Session.AutoRestore == nil || *got.Session.AutoRestore != false {
		t.Errorf("Session.AutoRestore = %v, want false", got.Session)
	}
	if got.Log == nil || got.Log.Level == nil || *got.Log.Level != "debug" {
		t.Errorf("Log.Level = %v, want debug", got.Log)
	}
}

func TestDaemonStatusResult_JSONRoundtrip(t *testing.T) {
	original := DaemonStatusResult{
		PID:                  12345,
		StartedAt:            "2026-02-11T10:00:00Z",
		Uptime:               "2h 30m",
		ConnectedClients:     2,
		ActiveSSHConnections: 3,
		ActiveForwards:       5,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal DaemonStatusResult: %v", err)
	}

	var got DaemonStatusResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal DaemonStatusResult: %v", err)
	}

	if got != original {
		t.Errorf("DaemonStatusResult roundtrip: got %+v, want %+v", got, original)
	}
}

func TestSSHEventNotification_JSONRoundtrip(t *testing.T) {
	original := SSHEventNotification{
		Type:  "error",
		Host:  "prod",
		Error: "connection refused",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal SSHEventNotification: %v", err)
	}

	var got SSHEventNotification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal SSHEventNotification: %v", err)
	}

	if got != original {
		t.Errorf("SSHEventNotification roundtrip: got %+v, want %+v", got, original)
	}
}

func TestSSHEventNotification_OmitsErrorWhenEmpty(t *testing.T) {
	notif := SSHEventNotification{Type: "connected", Host: "prod"}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Marshal SSHEventNotification: %v", err)
	}
	if strings.Contains(string(data), `"error"`) {
		t.Errorf("SSHEventNotification JSON should omit error when empty, got: %s", data)
	}
}

func TestForwardEventNotification_JSONRoundtrip(t *testing.T) {
	original := ForwardEventNotification{
		Type:  "error",
		Name:  "web",
		Host:  "prod",
		Error: "port in use",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal ForwardEventNotification: %v", err)
	}

	var got ForwardEventNotification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ForwardEventNotification: %v", err)
	}

	if got != original {
		t.Errorf("ForwardEventNotification roundtrip: got %+v, want %+v", got, original)
	}
}

func TestMetricsEventNotification_JSONRoundtrip(t *testing.T) {
	original := MetricsEventNotification{
		Sessions: []SessionMetrics{
			{
				Name:          "web",
				Status:        "active",
				BytesSent:     1024,
				BytesReceived: 2048,
				Uptime:        "1h 30m",
			},
			{
				Name:          "db",
				Status:        "active",
				BytesSent:     512,
				BytesReceived: 4096,
				Uptime:        "45m",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal MetricsEventNotification: %v", err)
	}

	var got MetricsEventNotification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal MetricsEventNotification: %v", err)
	}

	if len(got.Sessions) != 2 {
		t.Fatalf("len(Sessions) = %d, want 2", len(got.Sessions))
	}
	if got.Sessions[0] != original.Sessions[0] {
		t.Errorf("Sessions[0] = %+v, want %+v", got.Sessions[0], original.Sessions[0])
	}
	if got.Sessions[1] != original.Sessions[1] {
		t.Errorf("Sessions[1] = %+v, want %+v", got.Sessions[1], original.Sessions[1])
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
