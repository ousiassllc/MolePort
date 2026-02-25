package ssh

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestSSHManager_Subscribe_MultipleSubscribers(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	ch1 := sm.Subscribe()
	ch2 := sm.Subscribe()

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// 両方のサブスクライバーがイベントを受信
	for _, ch := range []<-chan core.SSHEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != core.SSHEventConnected {
				t.Errorf("event type = %v, want %v", ev.Type, core.SSHEventConnected)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestSSHManager_Close(t *testing.T) {
	hosts := testHosts()
	mockConn := &mockSSHConnection{client: nil, isAlive: true}
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return mockConn
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// drain connect event
	select {
	case <-events:
	case <-time.After(time.Second):
	}

	sm.Close()

	if sm.IsConnected("server1") {
		t.Error("server1 should be disconnected after Close")
	}

	// チャネルが閉じられていること
	_, ok := <-events
	if ok {
		t.Error("subscriber channel should be closed after Close")
	}
}

func TestSSHManager_Connect_ConcurrentSameHost(t *testing.T) {
	// 同一ホストへの並行 Connect が安全に動作し、接続ファクトリが1回しか呼ばれないことを確認する。
	hosts := testHosts()
	var callCount int
	var mu sync.Mutex

	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		mu.Lock()
		callCount++
		mu.Unlock()
		// 接続に少し時間がかかることをシミュレート
		mock := &mockSSHConnection{client: nil, isAlive: true}
		return mock
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = sm.Connect("server1")
		}(i)
	}
	wg.Wait()

	// エラーはないはず
	for i, err := range errs {
		if err != nil {
			t.Errorf("Connect goroutine[%d] error = %v", i, err)
		}
	}

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 1 {
		t.Errorf("connFactory called %d times, want 1", count)
	}

	sm.Close()
}

func TestSSHManager_Connect_AuthFailure_PendingAuth(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return &mockSSHConnection{
			dialErr: fmt.Errorf("ssh: handshake failed: ssh: unable to authenticate"),
			isAlive: false,
		}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	err := sm.Connect("server1")
	if err == nil {
		t.Fatal("Connect() should return error on auth failure")
	}

	// PendingAuth イベントを受信
	select {
	case evt := <-events:
		if evt.Type != core.SSHEventPendingAuth {
			t.Errorf("expected SSHEventPendingAuth, got %v", evt.Type)
		}
		if evt.HostName != "server1" {
			t.Errorf("expected host 'server1', got %q", evt.HostName)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for PendingAuth event")
	}

	// ホスト状態が PendingAuth であることを確認
	host, _ := sm.GetHost("server1")
	if host.State != core.PendingAuth {
		t.Errorf("expected PendingAuth state, got %v", host.State)
	}

	sm.Close()
}

func TestSSHManager_ConnectWithCallback_Success(t *testing.T) {
	hosts := testHosts()
	mockConn := &mockSSHConnection{client: nil, isAlive: true}
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return mockConn
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Value: "secret"}, nil
	}

	if err := sm.ConnectWithCallback("server1", cb); err != nil {
		t.Fatalf("ConnectWithCallback() error = %v", err)
	}

	if !sm.IsConnected("server1") {
		t.Error("server1 should be connected after ConnectWithCallback")
	}

	sm.Close()
}

func TestSSHManager_ConnectWithCallback_ClearsPendingAuth(t *testing.T) {
	hosts := testHosts()
	callCount := 0
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		callCount++
		if callCount == 1 {
			// 最初の Connect (cb=nil) は認証失敗
			return &mockSSHConnection{
				dialErr: fmt.Errorf("ssh: unable to authenticate"),
				isAlive: false,
			}
		}
		// 2回目 ConnectWithCallback は成功
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// まず Connect で PendingAuth にする
	_ = sm.Connect("server1")
	host, _ := sm.GetHost("server1")
	if host.State != core.PendingAuth {
		t.Fatalf("expected PendingAuth, got %v", host.State)
	}

	// ConnectWithCallback で接続成功
	cb := func(req core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{Value: "password"}, nil
	}
	if err := sm.ConnectWithCallback("server1", cb); err != nil {
		t.Fatalf("ConnectWithCallback() error = %v", err)
	}

	host, _ = sm.GetHost("server1")
	if host.State != core.Connected {
		t.Errorf("expected Connected, got %v", host.State)
	}

	pendingHosts := sm.GetPendingAuthHosts()
	if len(pendingHosts) != 0 {
		t.Errorf("expected 0 pending auth hosts, got %d", len(pendingHosts))
	}

	sm.Close()
}

func TestSSHManager_GetPendingAuthHosts(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return &mockSSHConnection{
			dialErr: fmt.Errorf("ssh: unable to authenticate"),
			isAlive: false,
		}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// 両方のホストに接続を試行（認証失敗 → PendingAuth）
	_ = sm.Connect("server1")
	_ = sm.Connect("server2")

	pendingHosts := sm.GetPendingAuthHosts()
	if len(pendingHosts) != 2 {
		t.Fatalf("expected 2 pending auth hosts, got %d", len(pendingHosts))
	}

	sm.Close()
}
