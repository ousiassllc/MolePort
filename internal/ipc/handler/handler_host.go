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
	before := h.sshMgr.GetHosts()
	beforeSet := make(map[string]struct{}, len(before))
	for _, host := range before {
		beforeSet[host.Name] = struct{}{}
	}

	after, err := h.sshMgr.ReloadHosts()
	if err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	afterSet := make(map[string]struct{}, len(after))
	for _, host := range after {
		afterSet[host.Name] = struct{}{}
	}

	var added []string
	for _, host := range after {
		if _, ok := beforeSet[host.Name]; !ok {
			added = append(added, host.Name)
		}
	}

	var removed []string
	for _, host := range before {
		if _, ok := afterSet[host.Name]; !ok {
			removed = append(removed, host.Name)
		}
	}

	if added == nil {
		added = []string{}
	}
	if removed == nil {
		removed = []string{}
	}

	return protocol.HostReloadResult{
		Total:   len(after),
		Added:   added,
		Removed: removed,
	}, nil
}
