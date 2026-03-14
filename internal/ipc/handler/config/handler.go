package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// parsedDurations はバリデーション時にパースした Duration を保持する。
type parsedDurations map[string]time.Duration

func validateAndParseDuration(value *string, fieldName string, parsed parsedDurations) *protocol.RPCError {
	if value == nil {
		return nil
	}
	d, err := time.ParseDuration(*value)
	if err != nil {
		return &protocol.RPCError{
			Code:    protocol.InvalidParams,
			Message: fmt.Sprintf("invalid %s: %s", fieldName, err),
		}
	}
	parsed[fieldName] = d
	return nil
}

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
		UpdateCheck: protocol.UpdateCheckInfo{
			Enabled:  cfg.UpdateCheck.Enabled,
			Interval: cfg.UpdateCheck.Interval.String(),
		},
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

	durations, rpcErr := validateParams(&p)
	if rpcErr != nil {
		return nil, rpcErr
	}

	if err := h.cfgMgr.UpdateConfig(func(cfg *core.Config) {
		if p.SSHConfigPath != nil {
			cfg.SSHConfigPath = *p.SSHConfigPath
		}
		applyReconnect(cfg, p.Reconnect, durations)
		applyHosts(cfg, p.Hosts, durations)
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
		if p.UpdateCheck != nil {
			if p.UpdateCheck.Enabled != nil {
				cfg.UpdateCheck.Enabled = *p.UpdateCheck.Enabled
			}
			if d, ok := durations["update_check.interval"]; ok {
				cfg.UpdateCheck.Interval = core.Duration{Duration: d}
			}
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

func validateParams(p *protocol.ConfigUpdateParams) (parsedDurations, *protocol.RPCError) {
	parsed := make(parsedDurations)
	if p.Reconnect != nil {
		if err := validateAndParseDuration(p.Reconnect.InitialDelay, "reconnect.initial_delay", parsed); err != nil {
			return nil, err
		}
		if err := validateAndParseDuration(p.Reconnect.MaxDelay, "reconnect.max_delay", parsed); err != nil {
			return nil, err
		}
		if err := validateAndParseDuration(p.Reconnect.KeepAliveInterval, "reconnect.keepalive_interval", parsed); err != nil {
			return nil, err
		}
	}
	if p.UpdateCheck != nil {
		if err := validateAndParseDuration(p.UpdateCheck.Interval, "update_check.interval", parsed); err != nil {
			return nil, err
		}
		if d, ok := parsed["update_check.interval"]; ok && d < time.Hour {
			return nil, &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: "update_check.interval must be at least 1h",
			}
		}
	}
	for name, update := range p.Hosts {
		if update == nil || update.Reconnect == nil {
			continue
		}
		if err := validateAndParseDuration(update.Reconnect.InitialDelay, "hosts."+name+".reconnect.initial_delay", parsed); err != nil {
			return nil, err
		}
		if err := validateAndParseDuration(update.Reconnect.MaxDelay, "hosts."+name+".reconnect.max_delay", parsed); err != nil {
			return nil, err
		}
	}
	return parsed, nil
}

func applyReconnect(cfg *core.Config, r *protocol.ReconnectUpdateInfo, durations parsedDurations) {
	if r == nil {
		return
	}
	if r.Enabled != nil {
		cfg.Reconnect.Enabled = *r.Enabled
	}
	if r.MaxRetries != nil {
		cfg.Reconnect.MaxRetries = *r.MaxRetries
	}
	if d, ok := durations["reconnect.initial_delay"]; ok {
		cfg.Reconnect.InitialDelay = core.Duration{Duration: d}
	}
	if d, ok := durations["reconnect.max_delay"]; ok {
		cfg.Reconnect.MaxDelay = core.Duration{Duration: d}
	}
	if d, ok := durations["reconnect.keepalive_interval"]; ok {
		cfg.Reconnect.KeepAliveInterval = core.Duration{Duration: d}
	}
}

func applyHosts(cfg *core.Config, hosts map[string]*protocol.HostConfigUpdateInfo, durations parsedDurations) {
	if hosts == nil {
		return
	}
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]core.HostConfig)
	}
	for name, update := range hosts {
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
			if d, ok := durations["hosts."+name+".reconnect.initial_delay"]; ok {
				hc.Reconnect.InitialDelay = &core.Duration{Duration: d}
			}
			if d, ok := durations["hosts."+name+".reconnect.max_delay"]; ok {
				hc.Reconnect.MaxDelay = &core.Duration{Duration: d}
			}
		}
		cfg.Hosts[name] = hc
	}
}
