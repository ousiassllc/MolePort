package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDaemonStatusResult_JSONRoundtrip(t *testing.T) {
	original := DaemonStatusResult{
		Version:              "v0.2.0",
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

	if !reflect.DeepEqual(got, original) {
		t.Errorf("DaemonStatusResult roundtrip: got %+v, want %+v", got, original)
	}
}

func TestDaemonStatusResult_WithWarnings(t *testing.T) {
	original := DaemonStatusResult{
		Version:              "v0.2.0",
		PID:                  12345,
		StartedAt:            "2026-02-11T10:00:00Z",
		Uptime:               "2h 30m",
		ConnectedClients:     2,
		ActiveSSHConnections: 3,
		ActiveForwards:       5,
		Warnings:             []string{"failed to load forward rule \"web\": port conflict", "failed to load SSH hosts: file not found"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal DaemonStatusResult: %v", err)
	}

	if !strings.Contains(string(data), `"warnings"`) {
		t.Errorf("DaemonStatusResult JSON should contain warnings, got: %s", data)
	}

	var got DaemonStatusResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal DaemonStatusResult: %v", err)
	}

	if !reflect.DeepEqual(got, original) {
		t.Errorf("DaemonStatusResult roundtrip: got %+v, want %+v", got, original)
	}
}

func TestDaemonStatusResult_OmitsEmptyWarnings(t *testing.T) {
	result := DaemonStatusResult{
		Version: "v0.2.0",
		PID:     12345,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal DaemonStatusResult: %v", err)
	}

	if strings.Contains(string(data), `"warnings"`) {
		t.Errorf("DaemonStatusResult JSON should omit empty warnings, got: %s", data)
	}
}
