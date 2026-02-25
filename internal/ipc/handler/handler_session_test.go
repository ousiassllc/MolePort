package handler

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_SessionList(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "session.list", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	sessionList, ok := result.(protocol.SessionListResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.SessionListResult", result)
	}

	if len(sessionList.Sessions) != 1 {
		t.Fatalf("sessions count = %d, want 1", len(sessionList.Sessions))
	}
	if sessionList.Sessions[0].Name != "web" {
		t.Errorf("sessions[0].Name = %q, want %q", sessionList.Sessions[0].Name, "web")
	}
	if sessionList.Sessions[0].Status != "active" {
		t.Errorf("sessions[0].Status = %q, want %q", sessionList.Sessions[0].Status, "active")
	}
	if sessionList.Sessions[0].BytesSent != 1024 {
		t.Errorf("sessions[0].BytesSent = %d, want %d", sessionList.Sessions[0].BytesSent, 1024)
	}
}

func TestHandler_SessionGet_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.SessionGetParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "session.get", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	sessionInfo, ok := result.(protocol.SessionInfo)
	if !ok {
		t.Fatalf("result type = %T, want protocol.SessionInfo", result)
	}
	if sessionInfo.Name != "web" {
		t.Errorf("Name = %q, want %q", sessionInfo.Name, "web")
	}
	if sessionInfo.Status != "active" {
		t.Errorf("Status = %q, want %q", sessionInfo.Status, "active")
	}
}

func TestHandler_SessionGet_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.SessionGetParams{Name: "nonexistent"})
	_, rpcErr := h.Handle("client-1", "session.get", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != protocol.RuleNotFound {
		t.Errorf("error code = %d, want %d (RuleNotFound)", rpcErr.Code, protocol.RuleNotFound)
	}
}
