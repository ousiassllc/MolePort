package handler

import (
	"testing"

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
