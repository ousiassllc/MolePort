package handler

import (
	"encoding/json"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func (h *Handler) sessionList() (any, *protocol.RPCError) {
	sessions := h.fwdMgr.GetAllSessions()

	result := protocol.SessionListResult{
		Sessions: make([]protocol.SessionInfo, len(sessions)),
	}
	for i, s := range sessions {
		result.Sessions[i] = protocol.ToSessionInfo(s)
	}
	return result, nil
}

func (h *Handler) sessionGet(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.SessionGetParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	session, err := h.fwdMgr.GetSession(p.Name)
	if err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	info := protocol.ToSessionInfo(*session)
	return info, nil
}
