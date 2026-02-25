package handler

import (
	"encoding/json"
	"log/slog"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func (h *Handler) daemonStatus() (any, *protocol.RPCError) {
	if h.daemon == nil {
		return nil, &protocol.RPCError{Code: protocol.InternalError, Message: "daemon not available"}
	}
	return h.daemon.Status(), nil
}

func (h *Handler) daemonShutdown(params json.RawMessage) (any, *protocol.RPCError) {
	if h.daemon == nil {
		return nil, &protocol.RPCError{Code: protocol.InternalError, Message: "daemon not available"}
	}

	var p protocol.DaemonShutdownParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			slog.Debug("daemonShutdown: invalid params, using defaults", "error", err)
		}
	}

	if err := h.daemon.Shutdown(p.Purge); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}
	return protocol.DaemonShutdownResult{OK: true}, nil
}
