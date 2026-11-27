package ssh

import (
	"context"
	"fmt"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestWithJitter(t *testing.T) {
	const sec = time.Second
	tests := []struct {
		name             string
		base, maxDelay   time.Duration
		wantMin, wantMax time.Duration
	}{
		{"first retry gets jitter", sec, 60 * sec, sec, sec + 100*time.Millisecond},
		{"never exceeds maxDelay", 60 * sec, 60 * sec, 60 * sec, 60 * sec},
		{"no cap when maxDelay is zero", 10 * sec, 0, 10 * sec, 11 * sec},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sawJitter := false
			for i := 0; i < 200; i++ {
				got := withJitter(tt.base, tt.maxDelay)
				if got < tt.wantMin || got > tt.wantMax {
					t.Fatalf("withJitter(%v, %v) = %v, want [%v, %v]", tt.base, tt.maxDelay, got, tt.wantMin, tt.wantMax)
				}
				sawJitter = sawJitter || got > tt.base
			}
			if tt.wantMax > tt.wantMin && !sawJitter {
				t.Error("jitter was never added")
			}
		})
	}
}

func TestNextBackoff(t *testing.T) {
	for _, tt := range []struct{ base, maxDelay, want time.Duration }{
		{time.Second, time.Minute, 2 * time.Second},
		{40 * time.Second, time.Minute, time.Minute},
		{40 * time.Second, 0, 80 * time.Second},
	} {
		if got := nextBackoff(tt.base, tt.maxDelay); got != tt.want {
			t.Errorf("nextBackoff(%v, %v) = %v, want %v", tt.base, tt.maxDelay, got, tt.want)
		}
	}
}

// max_retries: 0 は無制限（グローバル設定・ホスト別オーバーライドの両方）。
func TestSSHManager_Reconnect_MaxRetriesZeroIsUnlimited(t *testing.T) {
	zero := 0
	tests := []struct {
		name        string
		globalRetry int
		hostConfigs map[string]core.HostConfig
	}{
		{"global", 0, nil},
		{"host override", 1, map[string]core.HostConfig{"server1": {Reconnect: &core.ReconnectOverride{MaxRetries: &zero}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := make(chan struct{}, 100)
			calls := 0
			factory := func() core.SSHConnection {
				if calls++; calls == 1 {
					return &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
				}
				return &mockSSHConnection{dialF: func(core.SSHHost) (*cryptossh.Client, error) {
					attempts <- struct{}{}
					return nil, fmt.Errorf("connection refused")
				}}
			}
			sm := NewSSHManager(context.Background(), &mockSSHConfigParser{hosts: testHosts()}, factory, "/fake/ssh/config",
				core.ReconnectConfig{
					Enabled:      true,
					MaxRetries:   tt.globalRetry,
					InitialDelay: core.Duration{Duration: time.Millisecond},
					MaxDelay:     core.Duration{Duration: time.Millisecond},
				}, tt.hostConfigs)
			if _, err := sm.LoadHosts(); err != nil {
				t.Fatalf("LoadHosts() error = %v", err)
			}
			t.Cleanup(sm.Close)
			events := sm.Subscribe()
			if err := sm.Connect("server1"); err != nil {
				t.Fatalf("Connect() error = %v", err)
			}

			// 有限設定なら到達しない回数まで再接続を試行し続ける（Error イベントが出たら諦めた証拠）
			for i := 0; i < 5; {
				select {
				case <-attempts:
					i++
				case ev := <-events:
					if ev.Type == core.SSHEventError {
						t.Fatalf("reconnect gave up after %d attempts: %v", i, ev.Error)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("timeout waiting for reconnect attempt %d", i+1)
				}
			}
		})
	}
}
