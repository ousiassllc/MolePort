package handler

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_MethodNotFound(t *testing.T) {
	h, _, _, _ := newTestHandler()

	_, rpcErr := h.Handle("client-1", "nonexistent.method", nil)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != protocol.MethodNotFound {
		t.Errorf("error code = %d, want %d (MethodNotFound)", rpcErr.Code, protocol.MethodNotFound)
	}
}
