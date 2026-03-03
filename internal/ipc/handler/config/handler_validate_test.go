package config

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func strPtr(s string) *string { return &s }

func TestUpdate_InvalidDuration(t *testing.T) {
	tests := []struct {
		name   string
		params protocol.ConfigUpdateParams
	}{
		{
			name: "invalid reconnect.initial_delay",
			params: protocol.ConfigUpdateParams{
				Reconnect: &protocol.ReconnectUpdateInfo{
					InitialDelay: strPtr("not-a-duration"),
				},
			},
		},
		{
			name: "invalid reconnect.max_delay",
			params: protocol.ConfigUpdateParams{
				Reconnect: &protocol.ReconnectUpdateInfo{
					MaxDelay: strPtr("xyz"),
				},
			},
		},
		{
			name: "invalid reconnect.keepalive_interval",
			params: protocol.ConfigUpdateParams{
				Reconnect: &protocol.ReconnectUpdateInfo{
					KeepAliveInterval: strPtr("abc"),
				},
			},
		},
		{
			name: "invalid host reconnect.initial_delay",
			params: protocol.ConfigUpdateParams{
				Hosts: map[string]*protocol.HostConfigUpdateInfo{
					"prod": {
						Reconnect: &protocol.ReconnectUpdateInfo{
							InitialDelay: strPtr("bad"),
						},
					},
				},
			},
		},
		{
			name: "invalid host reconnect.max_delay",
			params: protocol.ConfigUpdateParams{
				Hosts: map[string]*protocol.HostConfigUpdateInfo{
					"prod": {
						Reconnect: &protocol.ReconnectUpdateInfo{
							MaxDelay: strPtr("bad"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler()
			params := mustMarshal(t, tt.params)
			_, rpcErr := h.Update(params)
			if rpcErr == nil {
				t.Fatal("expected RPC error for invalid duration")
			}
			if rpcErr.Code != protocol.InvalidParams {
				t.Errorf("error code = %d, want %d (InvalidParams)", rpcErr.Code, protocol.InvalidParams)
			}
		})
	}
}
