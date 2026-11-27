package config

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

type recordingApplier struct{ calls []bool }

func (r *recordingApplier) SetEnabled(enabled bool) { r.calls = append(r.calls, enabled) }

// update_check.enabled の変更は稼働中のチェッカーへ反映される（NFR-30）。
func TestUpdate_UpdateCheckEnabledAppliedToChecker(t *testing.T) {
	cfgMgr := &mockConfigManager{}
	applier := &recordingApplier{}
	h := New(cfgMgr, applier)

	for _, enabled := range []bool{false, true} {
		params := mustMarshal(t, protocol.ConfigUpdateParams{
			UpdateCheck: &protocol.UpdateCheckUpdateInfo{Enabled: &enabled},
		})
		if _, rpcErr := h.Update(params); rpcErr != nil {
			t.Fatalf("unexpected error: %v", rpcErr)
		}
	}

	if want := []bool{false, true}; !reflect.DeepEqual(applier.calls, want) {
		t.Errorf("SetEnabled calls = %v, want %v", applier.calls, want)
	}
}

// enabled を含まない更新、保存に失敗した更新ではチェッカーを変更しない。
func TestUpdate_UpdateCheckNotAppliedWhenUnchangedOrFailed(t *testing.T) {
	cfgMgr := &mockConfigManager{}
	applier := &recordingApplier{}
	h := New(cfgMgr, applier)

	interval := "48h"
	params := mustMarshal(t, protocol.ConfigUpdateParams{
		UpdateCheck: &protocol.UpdateCheckUpdateInfo{Interval: &interval},
	})
	if _, rpcErr := h.Update(params); rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	cfgMgr.err = errors.New("disk full")
	enabled := false
	params = mustMarshal(t, protocol.ConfigUpdateParams{
		UpdateCheck: &protocol.UpdateCheckUpdateInfo{Enabled: &enabled},
	})
	if _, rpcErr := h.Update(params); rpcErr == nil {
		t.Fatal("expected error when UpdateConfig fails")
	}

	if len(applier.calls) != 0 {
		t.Errorf("SetEnabled calls = %v, want none", applier.calls)
	}
}
