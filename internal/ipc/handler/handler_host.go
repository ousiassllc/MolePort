package handler

import (
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func (h *Handler) hostList() (any, *protocol.RPCError) {
	hosts, err := h.sshMgr.LoadHosts()
	if err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	result := protocol.HostListResult{
		Hosts: make([]protocol.HostInfo, len(hosts)),
	}
	for i, host := range hosts {
		result.Hosts[i] = protocol.ToHostInfo(host)
	}
	return result, nil
}

func (h *Handler) hostReload() (any, *protocol.RPCError) {
	// TODO: ReloadHosts 前後の差分を計算して Added/Removed を返す
	hosts, err := h.sshMgr.ReloadHosts()
	if err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.HostReloadResult{
		Total:   len(hosts),
		Added:   []string{},
		Removed: []string{},
	}, nil
}
