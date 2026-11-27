package ssh

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func boolPtr(b bool) *bool                  { return &b }
func intPtr(i int) *int                     { return &i }
func durPtr(d time.Duration) *core.Duration { return &core.Duration{Duration: d} }

func TestResolveReconnectConfig(t *testing.T) {
	global := core.ReconnectConfig{
		Enabled:      true,
		MaxRetries:   5,
		InitialDelay: core.Duration{Duration: 1 * time.Second},
		MaxDelay:     core.Duration{Duration: 30 * time.Second},
	}

	tests := []struct {
		name     string
		override *core.ReconnectOverride
		want     core.ReconnectConfig
	}{
		{
			name:     "nil override returns global unchanged",
			override: nil,
			want:     global,
		},
		{
			name:     "override Enabled only",
			override: &core.ReconnectOverride{Enabled: boolPtr(false)},
			want: core.ReconnectConfig{
				Enabled:      false,
				MaxRetries:   5,
				InitialDelay: core.Duration{Duration: 1 * time.Second},
				MaxDelay:     core.Duration{Duration: 30 * time.Second},
			},
		},
		{
			name:     "override MaxRetries only",
			override: &core.ReconnectOverride{MaxRetries: intPtr(10)},
			want: core.ReconnectConfig{
				Enabled:      true,
				MaxRetries:   10,
				InitialDelay: core.Duration{Duration: 1 * time.Second},
				MaxDelay:     core.Duration{Duration: 30 * time.Second},
			},
		},
		{
			name: "override all fields",
			override: &core.ReconnectOverride{
				Enabled:      boolPtr(false),
				MaxRetries:   intPtr(2),
				InitialDelay: durPtr(500 * time.Millisecond),
				MaxDelay:     durPtr(10 * time.Second),
			},
			want: core.ReconnectConfig{
				Enabled:      false,
				MaxRetries:   2,
				InitialDelay: core.Duration{Duration: 500 * time.Millisecond},
				MaxDelay:     core.Duration{Duration: 10 * time.Second},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReconnectConfig(global, tt.override)
			if got.Enabled != tt.want.Enabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tt.want.Enabled)
			}
			if got.MaxRetries != tt.want.MaxRetries {
				t.Errorf("MaxRetries = %v, want %v", got.MaxRetries, tt.want.MaxRetries)
			}
			if got.InitialDelay != tt.want.InitialDelay {
				t.Errorf("InitialDelay = %v, want %v", got.InitialDelay, tt.want.InitialDelay)
			}
			if got.MaxDelay != tt.want.MaxDelay {
				t.Errorf("MaxDelay = %v, want %v", got.MaxDelay, tt.want.MaxDelay)
			}
		})
	}
}

func TestSSHManager_PerHostReconnectDisabled(t *testing.T) {
	// グローバルで再接続有効だが、ホスト別に無効にした場合、再接続をスキップする。
	var connectCount atomic.Int32
	sm := NewSSHManager(context.Background(), &mockSSHConfigParser{hosts: testHosts()},
		func() core.SSHConnection {
			connectCount.Add(1)
			// KeepAlive がすぐに返ることで切断をシミュレート
			return &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
		},
		"/fake/ssh/config", fastReconnectConfig(3),
		map[string]core.HostConfig{"server1": {Reconnect: &core.ReconnectOverride{Enabled: boolPtr(false)}}},
	)
	defer sm.Close()
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}
	events := sm.Subscribe()
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	for _, want := range []core.SSHEventType{core.SSHEventConnected, core.SSHEventDisconnected} {
		ev := recvWithTimeout(t, events, want.String())
		if ev.Type != want {
			t.Fatalf("expected %v, got %v", want, ev.Type)
		}
		if want == core.SSHEventDisconnected && !ev.ReconnectDisabled {
			t.Error("Disconnected event ReconnectDisabled = false, want true")
		}
	}
	// Reconnecting イベントが来ないことを確認（再接続がスキップされる）
	select {
	case ev := <-events:
		t.Fatalf("expected no more events, got %v", ev.Type)
	case <-time.After(100 * time.Millisecond):
	}
	if got := connectCount.Load(); got != 1 {
		t.Errorf("connectCount = %d, want 1 (no reconnect attempts)", got)
	}
}
