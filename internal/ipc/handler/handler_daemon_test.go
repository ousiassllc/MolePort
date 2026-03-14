package handler

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_DaemonStatus(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "daemon.status", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	statusResult, ok := result.(protocol.DaemonStatusResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.DaemonStatusResult", result)
	}
	if statusResult.Version != "test" {
		t.Errorf("Version = %q, want %q", statusResult.Version, "test")
	}
	if statusResult.PID != 1234 {
		t.Errorf("PID = %d, want %d", statusResult.PID, 1234)
	}
	if statusResult.ConnectedClients != 2 {
		t.Errorf("ConnectedClients = %d, want %d", statusResult.ConnectedClients, 2)
	}
	if len(statusResult.Warnings) != 1 || statusResult.Warnings[0] != "test warning" {
		t.Errorf("Warnings = %v, want [\"test warning\"]", statusResult.Warnings)
	}
}

func TestHandler_DaemonStatus_NilDaemon(t *testing.T) {
	sender := func(_ string, _ protocol.Notification) error { return nil }
	broker := ipc.NewEventBroker(sender)
	h := NewHandler(&mockSSHManager{}, &mockForwardManager{}, &mockConfigManager{}, broker, nil, nil)

	_, rpcErr := h.Handle("client-1", "daemon.status", nil)
	if rpcErr == nil {
		t.Fatal("expected RPC error when daemon is nil")
	}
	if rpcErr.Code != protocol.InternalError {
		t.Errorf("error code = %d, want %d", rpcErr.Code, protocol.InternalError)
	}
}

func TestHandler_DaemonShutdown(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "daemon.shutdown", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	shutdownResult, ok := result.(protocol.DaemonShutdownResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.DaemonShutdownResult", result)
	}
	if !shutdownResult.OK {
		t.Error("OK should be true")
	}
}

func TestHandler_DaemonShutdown_Purge(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.DaemonShutdownParams{Purge: true})
	result, rpcErr := h.Handle("client-1", "daemon.shutdown", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	shutdownResult, ok := result.(protocol.DaemonShutdownResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.DaemonShutdownResult", result)
	}
	if !shutdownResult.OK {
		t.Error("OK should be true")
	}

	// handler 内で daemon.Shutdown(true) が呼ばれたことを検証するには
	// newTestHandler で作成した mockDaemonInfo を取り出す必要がある。
	// newTestHandler は daemon を直接返さないため、別途テストを構成する。
}

func TestHandler_DaemonShutdown_PurgeFlag(t *testing.T) {
	sshMgr := &mockSSHManager{}
	fwdMgr := &mockForwardManager{}
	cfgMgr := &mockConfigManager{}
	sender := func(_ string, _ protocol.Notification) error { return nil }
	broker := ipc.NewEventBroker(sender)
	daemonMock := &mockDaemonInfo{}

	handler := NewHandler(sshMgr, fwdMgr, cfgMgr, broker, daemonMock, nil)

	params := mustMarshal(t, protocol.DaemonShutdownParams{Purge: true})
	_, rpcErr := handler.Handle("client-1", "daemon.shutdown", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	if !daemonMock.lastPurgeFlag {
		t.Error("Shutdown should have been called with purge=true")
	}
}
