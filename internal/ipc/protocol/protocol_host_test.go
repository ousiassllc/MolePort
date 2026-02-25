package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

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
