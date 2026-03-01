package config

import (
	"encoding/json"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// Handler は設定関連の JSON-RPC メソッドを処理する。
type Handler struct {
	cfgMgr core.ConfigManager
}

// New は新しい設定ハンドラを生成する。
func New(cfgMgr core.ConfigManager) *Handler {
	return &Handler{cfgMgr: cfgMgr}
}

// Get は config.get リクエストを処理する。
func (h *Handler) Get() (any, *protocol.RPCError) {
	cfg := h.cfgMgr.GetConfig()

	result := protocol.ConfigGetResult{
		SSHConfigPath: cfg.SSHConfigPath,
		Reconnect: protocol.ReconnectInfo{
			Enabled:           cfg.Reconnect.Enabled,
			MaxRetries:        cfg.Reconnect.MaxRetries,
			InitialDelay:      cfg.Reconnect.InitialDelay.String(),
			MaxDelay:          cfg.Reconnect.MaxDelay.String(),
			KeepAliveInterval: cfg.Reconnect.KeepAliveInterval.String(),
		},
		Session: protocol.SessionCfgInfo{
			AutoRestore: cfg.Session.AutoRestore,
		},
		Log: protocol.LogInfo{
			Level: cfg.Log.Level,
			File:  cfg.Log.File,
		},
		Language: cfg.Language,
		TUI: protocol.TUIInfo{
			Theme: protocol.ThemeInfo{
				Base:   cfg.TUI.Theme.Base,
				Accent: cfg.TUI.Theme.Accent,
			},
		},
	}

	if len(cfg.Hosts) > 0 {
		result.Hosts = make(map[string]protocol.HostConfigInfo, len(cfg.Hosts))
		for name, hc := range cfg.Hosts {
			info := protocol.HostConfigInfo{}
			if hc.Reconnect != nil {
				override := &protocol.ReconnectOverrideInfo{
					Enabled:    hc.Reconnect.Enabled,
					MaxRetries: hc.Reconnect.MaxRetries,
				}
				if hc.Reconnect.InitialDelay != nil {
					s := hc.Reconnect.InitialDelay.String()
					override.InitialDelay = &s
				}
				if hc.Reconnect.MaxDelay != nil {
					s := hc.Reconnect.MaxDelay.String()
					override.MaxDelay = &s
				}
				info.Reconnect = override
			}
			result.Hosts[name] = info
		}
	}

	return result, nil
}

// Update は config.update リクエストを処理する。
func (h *Handler) Update(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.ConfigUpdateParams
	if len(params) == 0 {
		return nil, &protocol.RPCError{Code: protocol.InvalidParams, Message: "params required"}
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &protocol.RPCError{Code: protocol.InvalidParams, Message: "invalid params: " + err.Error()}
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
			if p.Reconnect.KeepAliveInterval != nil {
				if d, err := time.ParseDuration(*p.Reconnect.KeepAliveInterval); err == nil {
					cfg.Reconnect.KeepAliveInterval = core.Duration{Duration: d}
				}
			}
		}
		if p.Hosts != nil {
			if cfg.Hosts == nil {
				cfg.Hosts = make(map[string]core.HostConfig)
			}
			for name, update := range p.Hosts {
				if update == nil {
					delete(cfg.Hosts, name)
					continue
				}
				hc := cfg.Hosts[name]
				if update.Reconnect != nil {
					if hc.Reconnect == nil {
						hc.Reconnect = &core.ReconnectOverride{}
					}
					if update.Reconnect.Enabled != nil {
						hc.Reconnect.Enabled = update.Reconnect.Enabled
					}
					if update.Reconnect.MaxRetries != nil {
						hc.Reconnect.MaxRetries = update.Reconnect.MaxRetries
					}
					if update.Reconnect.InitialDelay != nil {
						if d, err := time.ParseDuration(*update.Reconnect.InitialDelay); err == nil {
							hc.Reconnect.InitialDelay = &core.Duration{Duration: d}
						}
					}
					if update.Reconnect.MaxDelay != nil {
						if d, err := time.ParseDuration(*update.Reconnect.MaxDelay); err == nil {
							hc.Reconnect.MaxDelay = &core.Duration{Duration: d}
						}
					}
				}
				cfg.Hosts[name] = hc
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
		if p.Language != nil {
			cfg.Language = *p.Language
		}
		if p.TUI != nil && p.TUI.Theme != nil {
			if p.TUI.Theme.Base != nil {
				cfg.TUI.Theme.Base = *p.TUI.Theme.Base
			}
			if p.TUI.Theme.Accent != nil {
				cfg.TUI.Theme.Accent = *p.TUI.Theme.Accent
			}
		}
	}); err != nil {
		return nil, protocol.ToRPCError(err, protocol.InternalError)
	}

	return protocol.ConfigUpdateResult{OK: true}, nil
}
