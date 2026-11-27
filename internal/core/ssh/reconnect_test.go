package ssh

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

// firstConnDropsFactory は最初の接続だけ KeepAlive がすぐ戻る（切断検出）mock を返す connFactory を作る。
// 2 回目以降の接続は dial の結果を返す。
func firstConnDropsFactory(dial func(core.SSHHost) (*cryptossh.Client, error)) func() core.SSHConnection {
	var calls atomic.Int32
	return func() core.SSHConnection {
		if calls.Add(1) == 1 {
			return &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
		}
		return &mockSSHConnection{isAlive: true, dialF: dial}
	}
}

func fastReconnectConfig(maxRetries int) core.ReconnectConfig {
	return core.ReconnectConfig{
		Enabled:      true,
		MaxRetries:   maxRetries,
		InitialDelay: core.Duration{Duration: time.Millisecond},
		MaxDelay:     core.Duration{Duration: time.Millisecond},
	}
}

func TestSSHManager_HandleDisconnect_WithReconnect(t *testing.T) {
	sm := newDialTestManager(t, firstConnDropsFactory(func(core.SSHHost) (*cryptossh.Client, error) { return nil, nil }),
		fastReconnectConfig(3))
	events := sm.Subscribe()
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	for i, want := range []core.SSHEventType{
		core.SSHEventConnected,    // 初回接続
		core.SSHEventDisconnected, // 切断検出
		core.SSHEventReconnecting, // 再接続開始
		core.SSHEventConnected,    // 再接続成功
	} {
		ev := recvWithTimeout(t, events, fmt.Sprintf("event[%d]", i))
		if ev.Type != want {
			t.Errorf("event[%d] type = %v, want %v", i, ev.Type, want)
		}
		if ev.ReconnectDisabled {
			t.Errorf("event[%d] ReconnectDisabled = true, want false", i)
		}
	}
}

func TestSSHManager_Disconnect_StopsReconnect(t *testing.T) {
	attempts := make(chan struct{}, 1000)
	sm := newDialTestManager(t, firstConnDropsFactory(func(core.SSHHost) (*cryptossh.Client, error) {
		attempts <- struct{}{}
		return nil, fmt.Errorf("simulated dial failure")
	}), fastReconnectConfig(1000))
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	recvWithTimeout(t, attempts, "first reconnect attempt")

	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	// 進行中だった試行は 1 回まで完了しうるが、それ以降は増えない
	time.Sleep(50 * time.Millisecond)
	before := len(attempts)
	time.Sleep(50 * time.Millisecond)
	if after := len(attempts); after != before {
		t.Errorf("reconnect continued after Disconnect: attempts went from %d to %d", before, after)
	}
}
