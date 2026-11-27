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
	tests := []struct {
		name     string
		base     time.Duration
		maxDelay time.Duration
		wantMin  time.Duration
		wantMax  time.Duration
	}{
		{
			name:     "first retry gets jitter",
			base:     1 * time.Second,
			maxDelay: 60 * time.Second,
			wantMin:  1 * time.Second,
			wantMax:  1*time.Second + 100*time.Millisecond, // +10%
		},
		{
			name:     "small delay",
			base:     20 * time.Millisecond,
			maxDelay: 1 * time.Second,
			wantMin:  20 * time.Millisecond,
			wantMax:  22 * time.Millisecond,
		},
		{
			name:     "never exceeds maxDelay",
			base:     60 * time.Second,
			maxDelay: 60 * time.Second,
			wantMin:  60 * time.Second,
			wantMax:  60 * time.Second,
		},
		{
			name:     "no cap when maxDelay is zero",
			base:     10 * time.Second,
			maxDelay: 0,
			wantMin:  10 * time.Second,
			wantMax:  11 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sawJitter := false
			for i := 0; i < 200; i++ {
				got := withJitter(tt.base, tt.maxDelay)
				if got < tt.wantMin || got > tt.wantMax {
					t.Fatalf("iteration %d: withJitter(%v, %v) = %v, want [%v, %v]",
						i, tt.base, tt.maxDelay, got, tt.wantMin, tt.wantMax)
				}
				if got > tt.base {
					sawJitter = true
				}
			}
			if tt.wantMax > tt.wantMin && !sawJitter {
				t.Errorf("withJitter(%v, %v) never added jitter in 200 iterations", tt.base, tt.maxDelay)
			}
		})
	}
}

func TestNextBackoff(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Duration
		maxDelay time.Duration
		want     time.Duration
	}{
		{"doubles", 1 * time.Second, 60 * time.Second, 2 * time.Second},
		{"capped by maxDelay", 40 * time.Second, 60 * time.Second, 60 * time.Second},
		{"no cap when maxDelay is zero", 40 * time.Second, 0, 80 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextBackoff(tt.base, tt.maxDelay); got != tt.want {
				t.Errorf("nextBackoff(%v, %v) = %v, want %v", tt.base, tt.maxDelay, got, tt.want)
			}
		})
	}
}

func TestSSHManager_Reconnect_MaxRetriesZeroIsUnlimited(t *testing.T) {
	zero := 0
	tests := []struct {
		name        string
		globalRetry int
		hostConfigs map[string]core.HostConfig
	}{
		{name: "global max_retries 0", globalRetry: 0},
		{
			name:        "host override max_retries 0",
			globalRetry: 1,
			hostConfigs: map[string]core.HostConfig{
				"server1": {Reconnect: &core.ReconnectOverride{MaxRetries: &zero}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const wantAttempts = 5 // 有限設定なら到達しない回数
			attempts := make(chan struct{}, 100)
			calls := 0
			factory := func() core.SSHConnection {
				calls++
				if calls == 1 {
					// 最初の接続は KeepAlive がすぐに戻る = 切断検出
					return &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
				}
				return &mockSSHConnection{dialF: func(core.SSHHost) (*cryptossh.Client, error) {
					attempts <- struct{}{}
					return nil, fmt.Errorf("connection refused")
				}}
			}
			sm := NewSSHManager(
				context.Background(),
				&mockSSHConfigParser{hosts: testHosts()},
				factory,
				"/fake/ssh/config",
				core.ReconnectConfig{
					Enabled:      true,
					MaxRetries:   tt.globalRetry,
					InitialDelay: core.Duration{Duration: time.Millisecond},
					MaxDelay:     core.Duration{Duration: time.Millisecond},
				},
				tt.hostConfigs,
			)
			if _, err := sm.LoadHosts(); err != nil {
				t.Fatalf("LoadHosts() error = %v", err)
			}
			t.Cleanup(sm.Close)
			events := sm.Subscribe()

			if err := sm.Connect("server1"); err != nil {
				t.Fatalf("Connect() error = %v", err)
			}

			for i := 0; i < wantAttempts; i++ {
				select {
				case <-attempts:
				case ev := <-events:
					if ev.Type == core.SSHEventError {
						t.Fatalf("reconnect gave up after %d attempts: %v", i, ev.Error)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("timeout waiting for reconnect attempt %d", i+1)
				}
			}

			// Disconnect で再接続ループが止まる
			if err := sm.Disconnect("server1"); err != nil {
				t.Fatalf("Disconnect() error = %v", err)
			}
		})
	}
}
