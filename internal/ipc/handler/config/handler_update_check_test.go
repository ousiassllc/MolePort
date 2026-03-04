package config

import (
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestGet_WithUpdateCheck(t *testing.T) {
	h, cfgMgr := newTestHandler()

	cfg := core.DefaultConfig()
	cfg.UpdateCheck.Enabled = false
	cfg.UpdateCheck.Interval = core.Duration{Duration: 48 * time.Hour}
	cfgMgr.config = &cfg

	result, rpcErr := h.Get()
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgResult := result.(protocol.ConfigGetResult)
	if cfgResult.UpdateCheck.Enabled != false {
		t.Errorf("UpdateCheck.Enabled = %v, want false", cfgResult.UpdateCheck.Enabled)
	}
	if cfgResult.UpdateCheck.Interval != "48h0m0s" {
		t.Errorf("UpdateCheck.Interval = %q, want %q", cfgResult.UpdateCheck.Interval, "48h0m0s")
	}
}

func TestGet_UpdateCheckDefaults(t *testing.T) {
	h, _ := newTestHandler()

	result, rpcErr := h.Get()
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgResult := result.(protocol.ConfigGetResult)
	if cfgResult.UpdateCheck.Enabled != true {
		t.Errorf("UpdateCheck.Enabled = %v, want true", cfgResult.UpdateCheck.Enabled)
	}
	if cfgResult.UpdateCheck.Interval != "24h0m0s" {
		t.Errorf("UpdateCheck.Interval = %q, want %q", cfgResult.UpdateCheck.Interval, "24h0m0s")
	}
}

func TestUpdate_UpdateCheck(t *testing.T) {
	h, cfgMgr := newTestHandler()

	enabled := false
	interval := "48h"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		UpdateCheck: &protocol.UpdateCheckUpdateInfo{
			Enabled:  &enabled,
			Interval: &interval,
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
	if cfg.UpdateCheck.Enabled != false {
		t.Errorf("UpdateCheck.Enabled = %v, want false", cfg.UpdateCheck.Enabled)
	}
	if cfg.UpdateCheck.Interval.Duration != 48*time.Hour {
		t.Errorf("UpdateCheck.Interval = %v, want 48h", cfg.UpdateCheck.Interval.Duration)
	}
}

func TestUpdate_UpdateCheckPartial(t *testing.T) {
	h, cfgMgr := newTestHandler()

	// Enabled のみ更新（Interval はデフォルトのまま）
	enabled := false
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		UpdateCheck: &protocol.UpdateCheckUpdateInfo{
			Enabled: &enabled,
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
	if cfg.UpdateCheck.Enabled != false {
		t.Errorf("UpdateCheck.Enabled = %v, want false", cfg.UpdateCheck.Enabled)
	}
	// Interval はデフォルト値のまま
	if cfg.UpdateCheck.Interval.Duration != 24*time.Hour {
		t.Errorf("UpdateCheck.Interval = %v, want 24h (unchanged)", cfg.UpdateCheck.Interval.Duration)
	}
}

func TestUpdate_UpdateCheckInvalidInterval(t *testing.T) {
	h, _ := newTestHandler()

	invalid := "not-a-duration"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		UpdateCheck: &protocol.UpdateCheckUpdateInfo{
			Interval: &invalid,
		},
	})

	_, rpcErr := h.Update(params)
	if rpcErr == nil {
		t.Fatal("expected RPC error for invalid duration")
	}
	if rpcErr.Code != protocol.InvalidParams {
		t.Errorf("error code = %d, want %d (InvalidParams)", rpcErr.Code, protocol.InvalidParams)
	}
}

func TestUpdate_UpdateCheckIntervalTooShort(t *testing.T) {
	h, _ := newTestHandler()

	short := "30m"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		UpdateCheck: &protocol.UpdateCheckUpdateInfo{
			Interval: &short,
		},
	})

	_, rpcErr := h.Update(params)
	if rpcErr == nil {
		t.Fatal("expected RPC error for interval < 1h")
	}
	if rpcErr.Code != protocol.InvalidParams {
		t.Errorf("error code = %d, want %d (InvalidParams)", rpcErr.Code, protocol.InvalidParams)
	}
}
