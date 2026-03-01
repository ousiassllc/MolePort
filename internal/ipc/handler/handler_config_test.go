package handler

import (
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_ConfigGet(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "config.get", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgResult, ok := result.(protocol.ConfigGetResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ConfigGetResult", result)
	}

	if cfgResult.SSHConfigPath != "~/.ssh/config" {
		t.Errorf("SSHConfigPath = %q, want %q", cfgResult.SSHConfigPath, "~/.ssh/config")
	}
	if !cfgResult.Reconnect.Enabled {
		t.Error("Reconnect.Enabled should be true")
	}
	if cfgResult.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfgResult.Log.Level, "info")
	}
	if cfgResult.Reconnect.KeepAliveInterval != "30s" {
		t.Errorf("Reconnect.KeepAliveInterval = %q, want %q", cfgResult.Reconnect.KeepAliveInterval, "30s")
	}
	// デフォルトではテーマは空文字列
	if cfgResult.TUI.Theme.Base != "" {
		t.Errorf("TUI.Theme.Base = %q, want empty string", cfgResult.TUI.Theme.Base)
	}
	if cfgResult.TUI.Theme.Accent != "" {
		t.Errorf("TUI.Theme.Accent = %q, want empty string", cfgResult.TUI.Theme.Accent)
	}
}

func TestHandler_ConfigUpdate(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	level := "debug"
	file := "/tmp/test.log"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		Log: &protocol.LogUpdateInfo{Level: &level, File: &file},
	})

	result, rpcErr := h.Handle("client-1", "config.update", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	updateResult, ok := result.(protocol.ConfigUpdateResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ConfigUpdateResult", result)
	}
	if !updateResult.OK {
		t.Error("OK should be true")
	}

	// 設定が更新されていることを確認
	cfg := cfgMgr.GetConfig()
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.File != "/tmp/test.log" {
		t.Errorf("Log.File = %q, want %q", cfg.Log.File, "/tmp/test.log")
	}
}

func TestHandler_ConfigGet_WithHosts(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	enabled := true
	maxRetries := 5
	cfg := core.DefaultConfig()
	cfg.Hosts = map[string]core.HostConfig{
		"prod": {
			Reconnect: &core.ReconnectOverride{
				Enabled:    &enabled,
				MaxRetries: &maxRetries,
				MaxDelay:   &core.Duration{Duration: 120 * time.Second},
			},
		},
	}
	cfgMgr.config = &cfg

	result, rpcErr := h.Handle("client-1", "config.get", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgResult := result.(protocol.ConfigGetResult)

	if len(cfgResult.Hosts) != 1 {
		t.Fatalf("len(Hosts) = %d, want 1", len(cfgResult.Hosts))
	}
	prod, ok := cfgResult.Hosts["prod"]
	if !ok {
		t.Fatal("Hosts[\"prod\"] not found")
	}
	if prod.Reconnect == nil {
		t.Fatal("Hosts[\"prod\"].Reconnect is nil")
	}
	if prod.Reconnect.Enabled == nil || *prod.Reconnect.Enabled != true {
		t.Errorf("Hosts[\"prod\"].Reconnect.Enabled = %v, want true", prod.Reconnect.Enabled)
	}
	if prod.Reconnect.MaxRetries == nil || *prod.Reconnect.MaxRetries != 5 {
		t.Errorf("Hosts[\"prod\"].Reconnect.MaxRetries = %v, want 5", prod.Reconnect.MaxRetries)
	}
	if prod.Reconnect.InitialDelay != nil {
		t.Errorf("Hosts[\"prod\"].Reconnect.InitialDelay should be nil, got %v", prod.Reconnect.InitialDelay)
	}
	if prod.Reconnect.MaxDelay == nil || *prod.Reconnect.MaxDelay != "2m0s" {
		t.Errorf("Hosts[\"prod\"].Reconnect.MaxDelay = %v, want \"2m0s\"", prod.Reconnect.MaxDelay)
	}
}

func TestHandler_ConfigUpdate_KeepAliveInterval(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	interval := "45s"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		Reconnect: &protocol.ReconnectUpdateInfo{
			KeepAliveInterval: &interval,
		},
	})

	result, rpcErr := h.Handle("client-1", "config.update", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	updateResult := result.(protocol.ConfigUpdateResult)
	if !updateResult.OK {
		t.Error("OK should be true")
	}

	cfg := cfgMgr.GetConfig()
	if cfg.Reconnect.KeepAliveInterval.Duration != 45*time.Second {
		t.Errorf("KeepAliveInterval = %v, want 45s", cfg.Reconnect.KeepAliveInterval.Duration)
	}
}

func TestHandler_ConfigUpdate_Hosts(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	enabled := true
	maxRetries := 3
	maxDelay := "2m"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		Hosts: map[string]*protocol.HostConfigUpdateInfo{
			"prod": {
				Reconnect: &protocol.ReconnectUpdateInfo{
					Enabled:    &enabled,
					MaxRetries: &maxRetries,
					MaxDelay:   &maxDelay,
				},
			},
		},
	})

	result, rpcErr := h.Handle("client-1", "config.update", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	updateResult := result.(protocol.ConfigUpdateResult)
	if !updateResult.OK {
		t.Error("OK should be true")
	}

	cfg := cfgMgr.GetConfig()
	hc, ok := cfg.Hosts["prod"]
	if !ok {
		t.Fatal("Hosts[\"prod\"] not found")
	}
	if hc.Reconnect == nil {
		t.Fatal("Hosts[\"prod\"].Reconnect is nil")
	}
	if hc.Reconnect.Enabled == nil || *hc.Reconnect.Enabled != true {
		t.Errorf("Enabled = %v, want true", hc.Reconnect.Enabled)
	}
	if hc.Reconnect.MaxRetries == nil || *hc.Reconnect.MaxRetries != 3 {
		t.Errorf("MaxRetries = %v, want 3", hc.Reconnect.MaxRetries)
	}
	if hc.Reconnect.MaxDelay == nil || hc.Reconnect.MaxDelay.Duration != 2*time.Minute {
		t.Errorf("MaxDelay = %v, want 2m0s", hc.Reconnect.MaxDelay)
	}
}

func TestHandler_ConfigUpdate_HostsDelete(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	// まずホスト設定を追加
	enabled := true
	cfg := core.DefaultConfig()
	cfg.Hosts = map[string]core.HostConfig{
		"prod": {
			Reconnect: &core.ReconnectOverride{
				Enabled: &enabled,
			},
		},
	}
	cfgMgr.config = &cfg

	// nil 値で削除
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		Hosts: map[string]*protocol.HostConfigUpdateInfo{
			"prod": nil,
		},
	})

	result, rpcErr := h.Handle("client-1", "config.update", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	updateResult := result.(protocol.ConfigUpdateResult)
	if !updateResult.OK {
		t.Error("OK should be true")
	}

	updatedCfg := cfgMgr.GetConfig()
	if _, ok := updatedCfg.Hosts["prod"]; ok {
		t.Error("Hosts[\"prod\"] should have been deleted")
	}
}

func TestHandler_ConfigGet_WithTheme(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	cfg := core.DefaultConfig()
	cfg.TUI.Theme.Base = "dark"
	cfg.TUI.Theme.Accent = "#FF6600"
	cfgMgr.config = &cfg

	result, rpcErr := h.Handle("client-1", "config.get", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgResult := result.(protocol.ConfigGetResult)
	if cfgResult.TUI.Theme.Base != "dark" {
		t.Errorf("TUI.Theme.Base = %q, want %q", cfgResult.TUI.Theme.Base, "dark")
	}
	if cfgResult.TUI.Theme.Accent != "#FF6600" {
		t.Errorf("TUI.Theme.Accent = %q, want %q", cfgResult.TUI.Theme.Accent, "#FF6600")
	}
}

func TestHandler_ConfigUpdate_Theme(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	base := "dark"
	accent := "#FF6600"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		TUI: &protocol.TUIUpdateInfo{
			Theme: &protocol.ThemeUpdateInfo{
				Base:   &base,
				Accent: &accent,
			},
		},
	})

	result, rpcErr := h.Handle("client-1", "config.update", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	updateResult := result.(protocol.ConfigUpdateResult)
	if !updateResult.OK {
		t.Error("OK should be true")
	}

	cfg := cfgMgr.GetConfig()
	if cfg.TUI.Theme.Base != "dark" {
		t.Errorf("TUI.Theme.Base = %q, want %q", cfg.TUI.Theme.Base, "dark")
	}
	if cfg.TUI.Theme.Accent != "#FF6600" {
		t.Errorf("TUI.Theme.Accent = %q, want %q", cfg.TUI.Theme.Accent, "#FF6600")
	}
}

func TestHandler_ConfigUpdate_ThemePartial(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	// まずテーマを設定
	cfg := core.DefaultConfig()
	cfg.TUI.Theme.Base = "dark"
	cfg.TUI.Theme.Accent = "#FF6600"
	cfgMgr.config = &cfg

	// Accent のみ更新
	newAccent := "#00FF00"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		TUI: &protocol.TUIUpdateInfo{
			Theme: &protocol.ThemeUpdateInfo{
				Accent: &newAccent,
			},
		},
	})

	result, rpcErr := h.Handle("client-1", "config.update", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	updateResult := result.(protocol.ConfigUpdateResult)
	if !updateResult.OK {
		t.Error("OK should be true")
	}

	updatedCfg := cfgMgr.GetConfig()
	if updatedCfg.TUI.Theme.Base != "dark" {
		t.Errorf("TUI.Theme.Base = %q, want %q (unchanged)", updatedCfg.TUI.Theme.Base, "dark")
	}
	if updatedCfg.TUI.Theme.Accent != "#00FF00" {
		t.Errorf("TUI.Theme.Accent = %q, want %q", updatedCfg.TUI.Theme.Accent, "#00FF00")
	}
}
