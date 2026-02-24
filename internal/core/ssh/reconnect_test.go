package ssh

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestSSHManager_HandleDisconnect_WithReconnect(t *testing.T) {
	hosts := testHosts()
	connectCount := 0
	var mu sync.Mutex

	parser := &mockSSHConfigParser{hosts: hosts}
	sm := NewSSHManager(
		parser,
		func() core.SSHConnection {
			mu.Lock()
			connectCount++
			count := connectCount
			mu.Unlock()

			mock := &mockSSHConnection{client: nil, isAlive: true}
			if count == 1 {
				// 最初の接続: KeepAlive がすぐに返ることで切断をシミュレート
				mock.keepAliveF = func(ctx context.Context, interval time.Duration) {
					// すぐに返る = 切断検出
				}
			}
			return mock
		},
		"/fake/ssh/config",
		core.ReconnectConfig{
			Enabled:      true,
			MaxRetries:   3,
			InitialDelay: core.Duration{Duration: 10 * time.Millisecond},
			MaxDelay:     core.Duration{Duration: 50 * time.Millisecond},
		},
	)

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// Connected -> Disconnected -> Reconnecting -> Connected の流れを確認
	expectedTypes := []core.SSHEventType{
		core.SSHEventConnected,    // 初回接続
		core.SSHEventDisconnected, // 切断検出
		core.SSHEventReconnecting, // 再接続開始
		core.SSHEventConnected,    // 再接続成功
	}

	for i, expected := range expectedTypes {
		select {
		case ev := <-events:
			if ev.Type != expected {
				t.Errorf("event[%d] type = %v, want %v", i, ev.Type, expected)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for event[%d] (%v)", i, expected)
		}
	}

	sm.Close()
}

func TestSSHManager_Disconnect_StopsReconnect(t *testing.T) {
	// Disconnect がホストの再接続ループを停止することを確認する。
	hosts := testHosts()
	var connectCount int
	var mu sync.Mutex

	parser := &mockSSHConfigParser{hosts: hosts}
	sm := NewSSHManager(
		parser,
		func() core.SSHConnection {
			mu.Lock()
			connectCount++
			count := connectCount
			mu.Unlock()

			mock := &mockSSHConnection{client: nil, isAlive: true}
			if count == 1 {
				// 最初の接続: KeepAlive がすぐに返ることで切断をシミュレート
				mock.keepAliveF = func(ctx context.Context, interval time.Duration) {
				}
			}
			// 2回目以降の接続（再接続試行）: Dial に少し時間がかかる
			if count > 1 {
				mock.dialErr = fmt.Errorf("simulated slow dial")
			}
			return mock
		},
		"/fake/ssh/config",
		core.ReconnectConfig{
			Enabled:      true,
			MaxRetries:   100, // 多めに設定
			InitialDelay: core.Duration{Duration: 10 * time.Millisecond},
			MaxDelay:     core.Duration{Duration: 50 * time.Millisecond},
		},
	)

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// Connected を待つ
	select {
	case ev := <-events:
		if ev.Type != core.SSHEventConnected {
			t.Fatalf("expected Connected, got %v", ev.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for connected event")
	}

	// Disconnected を待つ
	select {
	case ev := <-events:
		if ev.Type != core.SSHEventDisconnected {
			t.Fatalf("expected Disconnected, got %v", ev.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for disconnected event")
	}

	// Reconnecting を待つ
	select {
	case ev := <-events:
		if ev.Type != core.SSHEventReconnecting {
			t.Fatalf("expected Reconnecting, got %v", ev.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for reconnecting event")
	}

	// 再接続中に Disconnect を呼ぶ
	time.Sleep(30 * time.Millisecond) // 少し再接続を試みさせる
	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	// 再接続が停止されたことを確認: これ以上 connectCount が増えないことを検証
	mu.Lock()
	countAfterDisconnect := connectCount
	mu.Unlock()

	time.Sleep(200 * time.Millisecond) // 再接続が続いていれば増えるはず

	mu.Lock()
	countLater := connectCount
	mu.Unlock()

	if countLater > countAfterDisconnect+1 {
		t.Errorf("reconnect continued after Disconnect: count went from %d to %d",
			countAfterDisconnect, countLater)
	}

	sm.Close()
}
