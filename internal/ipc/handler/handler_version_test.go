package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

type mockVersionChecker struct {
	result *core.VersionCheckResult
	err    error

	enabledCalls []bool
}

func (m *mockVersionChecker) SetEnabled(enabled bool) {
	m.enabledCalls = append(m.enabledCalls, enabled)
}

func (m *mockVersionChecker) LatestVersion(_ context.Context) (*core.VersionCheckResult, error) {
	return m.result, m.err
}

func newVCHandler(ver string, enabled bool, vc VersionChecker) *Handler {
	cfg := &mockConfigManager{config: &core.Config{UpdateCheck: core.UpdateCheckConfig{Enabled: enabled}}}
	b := ipc.NewEventBroker(func(_ string, _ protocol.Notification) error { return nil })
	return NewHandler(&mockSSHManager{}, &mockForwardManager{}, cfg, b,
		&mockDaemonInfo{status: protocol.DaemonStatusResult{Version: ver}}, vc)
}

func TestHandler_VersionCheck(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name, ver, wantVer, wantLatest string
		enabled, wantUpdate, wantErr   bool
		vc                             VersionChecker
	}{
		{"update_available", "v1.0.0", "v1.0.0", "v1.2.0", true, true, false,
			&mockVersionChecker{result: &core.VersionCheckResult{
				LatestVersion: "v1.2.0", ReleaseURL: "https://example.com",
				CheckedAt: now, UpdateAvailable: true,
			}}},
		{"no_update", "v1.0.0", "v1.0.0", "v1.0.0", true, false, false,
			&mockVersionChecker{result: &core.VersionCheckResult{LatestVersion: "v1.0.0", CheckedAt: now}}},
		{"dev", "dev", "dev", "", true, false, false, nil},
		{"disabled", "v1.0.0", "v1.0.0", "", false, false, false, nil},
		{"nil_checker", "v1.0.0", "v1.0.0", "", true, false, false, nil},
		{"nil_result", "v1.0.0", "v1.0.0", "", true, false, false, &mockVersionChecker{}},
		{"error", "v1.0.0", "", "", true, false, true, &mockVersionChecker{err: fmt.Errorf("network error")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, rpcErr := newVCHandler(tt.ver, tt.enabled, tt.vc).Handle("client-1", "version.check", nil)
			if tt.wantErr {
				if rpcErr == nil {
					t.Fatal("expected RPC error")
				}
				return
			}
			if rpcErr != nil {
				t.Fatalf("unexpected error: %v", rpcErr)
			}
			vr := result.(protocol.VersionCheckResult)
			if vr.CurrentVersion != tt.wantVer {
				t.Errorf("CurrentVersion = %q, want %q", vr.CurrentVersion, tt.wantVer)
			}
			if vr.LatestVersion != tt.wantLatest {
				t.Errorf("LatestVersion = %q, want %q", vr.LatestVersion, tt.wantLatest)
			}
			if vr.UpdateAvailable != tt.wantUpdate {
				t.Errorf("UpdateAvailable = %v, want %v", vr.UpdateAvailable, tt.wantUpdate)
			}
		})
	}
}

// config.update の update_check.enabled は Handler に渡された VersionChecker へ反映される（NFR-30）。
func TestHandler_ConfigUpdate_AppliesUpdateCheckEnabledToVersionChecker(t *testing.T) {
	vc := &mockVersionChecker{}
	h := newVCHandler("v1.0.0", true, vc)

	for _, enabled := range []bool{false, true} {
		params := json.RawMessage(fmt.Sprintf(`{"update_check":{"enabled":%t}}`, enabled))
		if _, rpcErr := h.Handle("client", "config.update", params); rpcErr != nil {
			t.Fatalf("config.update error = %v", rpcErr)
		}
	}

	if want := []bool{false, true}; !reflect.DeepEqual(vc.enabledCalls, want) {
		t.Errorf("SetEnabled calls = %v, want %v", vc.enabledCalls, want)
	}
}
