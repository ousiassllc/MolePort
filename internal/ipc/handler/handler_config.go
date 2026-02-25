package handler

import (
	"encoding/json"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func (h *Handler) configGet() (any, *protocol.RPCError) {
	cfg := h.cfgMgr.GetConfig()

	return protocol.ConfigGetResult{
		SSHConfigPath: cfg.SSHConfigPath,
		Reconnect: protocol.ReconnectInfo{
			Enabled:      cfg.Reconnect.Enabled,
			MaxRetries:   cfg.Reconnect.MaxRetries,
			InitialDelay: cfg.Reconnect.InitialDelay.String(),
			MaxDelay:     cfg.Reconnect.MaxDelay.String(),
		},
		Session: protocol.SessionCfgInfo{
			AutoRestore: cfg.Session.AutoRestore,
		},
		Log: protocol.LogInfo{
			Level: cfg.Log.Level,
			File:  cfg.Log.File,
		},
	}, nil
}

func (h *Handler) configUpdate(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.ConfigUpdateParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if err := h.cfgMgr.UpdateConfig(func(cfg *core.Config) {
		if p.SSHConfigPath != nil {
			cfg.SSHConfigPath = *p.SSHConfigPath
		}
		if p.Reconnect != nil {
			if p.Reconnect.Enabled != nil {
				cfg.Reconnect.Enabled = *p.Reconnect.Enabled
			}
			if p.Reconnect.MaxRetries != nil {
				cfg.Reconnect.MaxRetries = *p.Reconnect.MaxRetries
			}
			if p.Reconnect.InitialDelay != nil {
				if d, err := time.ParseDuration(*p.Reconnect.InitialDelay); err == nil {
					cfg.Reconnect.InitialDelay = core.Duration{Duration: d}
				}
			}
			if p.Reconnect.MaxDelay != nil {
				if d, err := time.ParseDuration(*p.Reconnect.MaxDelay); err == nil {
					cfg.Reconnect.MaxDelay = core.Duration{Duration: d}
				}
			}
		}
		if p.Session != nil && p.Session.AutoRestore != nil {
			cfg.Session.AutoRestore = *p.Session.AutoRestore
		}
		if p.Log != nil {
			if p.Log.Level != nil {
				cfg.Log.Level = *p.Log.Level
			}
			if p.Log.File != nil {
				cfg.Log.File = *p.Log.File
			}
		}
	}); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.ConfigUpdateResult{OK: true}, nil
}
