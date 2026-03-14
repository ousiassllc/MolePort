package config

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestGet_WithLanguage(t *testing.T) {
	h, cfgMgr := newTestHandler()

	cfg := core.DefaultConfig()
	cfg.Language = "ja"
	cfgMgr.config = &cfg

	result, rpcErr := h.Get()
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgResult := result.(protocol.ConfigGetResult)
	if cfgResult.Language != "ja" {
		t.Errorf("Language = %q, want %q", cfgResult.Language, "ja")
	}
}

func TestUpdate_Language(t *testing.T) {
	h, cfgMgr := newTestHandler()

	lang := "ja"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		Language: &lang,
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
	if cfg.Language != "ja" {
		t.Errorf("Language = %q, want %q", cfg.Language, "ja")
	}
}
