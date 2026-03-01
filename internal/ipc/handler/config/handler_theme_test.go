package config

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestGet_WithTheme(t *testing.T) {
	h, cfgMgr := newTestHandler()

	cfg := core.DefaultConfig()
	cfg.TUI.Theme.Base = "dark"
	cfg.TUI.Theme.Accent = "#FF6600"
	cfgMgr.config = &cfg

	result, rpcErr := h.Get()
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

func TestUpdate_Theme(t *testing.T) {
	h, cfgMgr := newTestHandler()

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

	result, rpcErr := h.Update(params)
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

func TestUpdate_ThemePartial(t *testing.T) {
	h, cfgMgr := newTestHandler()

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

	result, rpcErr := h.Update(params)
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
