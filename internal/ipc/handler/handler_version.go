package handler

import (
	"context"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func (h *Handler) versionCheck() (any, *protocol.RPCError) {
	ver := h.daemon.Status().Version
	if ver == "dev" || !h.cfgMgr.GetConfig().UpdateCheck.Enabled || h.versionChecker == nil {
		return protocol.VersionCheckResult{CurrentVersion: ver}, nil
	}
	result, err := h.versionChecker.LatestVersion(context.Background())
	if err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}
	resp := protocol.VersionCheckResult{CurrentVersion: ver}
	if result != nil {
		resp.LatestVersion = result.LatestVersion
		resp.UpdateAvailable = result.UpdateAvailable
		resp.ReleaseURL = result.ReleaseURL
		resp.CheckedAt = result.CheckedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return resp, nil
}
