package core

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// --- Mock SSHConfigParser ---

type mockSSHConfigParser struct {
	hosts []SSHHost
	err   error
}

func (m *mockSSHConfigParser) Parse(configPath string) ([]SSHHost, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([]SSHHost, len(m.hosts))
	copy(result, m.hosts)
	return result, nil
}

// --- Mock SSHConnection ---

type mockSSHConnection struct {
	mu         sync.Mutex
	dialErr    error
	client     *ssh.Client
	closed     bool
	isAlive    bool
	keepAliveF func(ctx context.Context, interval time.Duration)

	localForwardF   func(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)
	remoteForwardF  func(ctx context.Context, remotePort int, localAddr string) (net.Listener, error)
	dynamicForwardF func(ctx context.Context, localPort int) (net.Listener, error)
}

func (m *mockSSHConnection) Dial(host SSHHost) (*ssh.Client, error) {
	if m.dialErr != nil {
		return nil, m.dialErr
	}
	return m.client, nil
}

func (m *mockSSHConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockSSHConnection) LocalForward(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error) {
	if m.localForwardF != nil {
		return m.localForwardF(ctx, localPort, remoteAddr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) RemoteForward(ctx context.Context, remotePort int, localAddr string) (net.Listener, error) {
	if m.remoteForwardF != nil {
		return m.remoteForwardF(ctx, remotePort, localAddr)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) DynamicForward(ctx context.Context, localPort int) (net.Listener, error) {
	if m.dynamicForwardF != nil {
		return m.dynamicForwardF(ctx, localPort)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSSHConnection) IsAlive() bool {
	return m.isAlive
}

func (m *mockSSHConnection) KeepAlive(ctx context.Context, interval time.Duration) {
	if m.keepAliveF != nil {
		m.keepAliveF(ctx, interval)
		return
	}
	// デフォルト: コンテキストがキャンセルされるまでブロック
	<-ctx.Done()
}

// インターフェース適合チェック
var _ SSHConfigParser = (*mockSSHConfigParser)(nil)
var _ SSHConnection = (*mockSSHConnection)(nil)

// --- Tests ---

func testHosts() []SSHHost {
	return []SSHHost{
		{Name: "server1", HostName: "192.168.1.1", Port: 22, User: "user1", State: Disconnected},
		{Name: "server2", HostName: "192.168.1.2", Port: 2222, User: "user2", State: Disconnected},
	}
}

func newTestSSHManager(hosts []SSHHost, connFactory func() SSHConnection) SSHManager {
	parser := &mockSSHConfigParser{hosts: hosts}
	return NewSSHManager(parser, connFactory, "/fake/ssh/config", ReconnectConfig{
		Enabled:      false,
		MaxRetries:   3,
		InitialDelay: Duration{Duration: 10 * time.Millisecond},
		MaxDelay:     Duration{Duration: 50 * time.Millisecond},
	})
}

func TestSSHManager_LoadHosts(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, nil)

	loaded, err := sm.LoadHosts()
	if err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("len(hosts) = %d, want 2", len(loaded))
	}
	if loaded[0].Name != "server1" {
		t.Errorf("hosts[0].Name = %q, want %q", loaded[0].Name, "server1")
	}
	if loaded[1].Name != "server2" {
		t.Errorf("hosts[1].Name = %q, want %q", loaded[1].Name, "server2")
	}
}

func TestSSHManager_LoadHosts_ParseError(t *testing.T) {
	parser := &mockSSHConfigParser{err: fmt.Errorf("parse error")}
	sm := NewSSHManager(parser, nil, "/fake/ssh/config", ReconnectConfig{})

	_, err := sm.LoadHosts()
	if err == nil {
		t.Fatal("LoadHosts() should return error on parse failure")
	}
}

func TestSSHManager_GetHost(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, nil)

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	host, err := sm.GetHost("server1")
	if err != nil {
		t.Fatalf("GetHost() error = %v", err)
	}
	if host.Name != "server1" {
		t.Errorf("host.Name = %q, want %q", host.Name, "server1")
	}
	if host.HostName != "192.168.1.1" {
		t.Errorf("host.HostName = %q, want %q", host.HostName, "192.168.1.1")
	}
}

func TestSSHManager_GetHost_NotFound(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	_, err := sm.GetHost("nonexistent")
	if err == nil {
		t.Fatal("GetHost() should return error for nonexistent host")
	}
}

func TestSSHManager_Connect_Disconnect(t *testing.T) {
	hosts := testHosts()
	mockConn := &mockSSHConnection{client: nil, isAlive: true}

	sm := newTestSSHManager(hosts, func() SSHConnection {
		return mockConn
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if !sm.IsConnected("server1") {
		t.Error("server1 should be connected")
	}

	// 接続イベントを受信
	select {
	case ev := <-events:
		if ev.Type != SSHEventConnected {
			t.Errorf("event type = %v, want %v", ev.Type, SSHEventConnected)
		}
		if ev.HostName != "server1" {
			t.Errorf("event host = %q, want %q", ev.HostName, "server1")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for connect event")
	}

	host, _ := sm.GetHost("server1")
	if host.State != Connected {
		t.Errorf("host state = %v, want %v", host.State, Connected)
	}

	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	if sm.IsConnected("server1") {
		t.Error("server1 should be disconnected")
	}

	// 切断イベントを受信
	select {
	case ev := <-events:
		if ev.Type != SSHEventDisconnected {
			t.Errorf("event type = %v, want %v", ev.Type, SSHEventDisconnected)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for disconnect event")
	}
}

func TestSSHManager_Connect_AlreadyConnected(t *testing.T) {
	hosts := testHosts()
	callCount := 0
	sm := newTestSSHManager(hosts, func() SSHConnection {
		callCount++
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// 二回目の接続はスキップされる
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}

	if callCount != 1 {
		t.Errorf("connFactory called %d times, want 1", callCount)
	}
}

func TestSSHManager_Connect_DialError(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() SSHConnection {
		return &mockSSHConnection{dialErr: fmt.Errorf("connection refused")}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	events := sm.Subscribe()

	err := sm.Connect("server1")
	if err == nil {
		t.Fatal("Connect() should return error on dial failure")
	}

	// エラーイベント
	select {
	case ev := <-events:
		if ev.Type != SSHEventError {
			t.Errorf("event type = %v, want %v", ev.Type, SSHEventError)
		}
		if ev.Error == nil {
			t.Error("event error should not be nil")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error event")
	}

	host, _ := sm.GetHost("server1")
	if host.State != ConnectionError {
		t.Errorf("host state = %v, want %v", host.State, ConnectionError)
	}
}

func TestSSHManager_Connect_HostNotFound(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	err := sm.Connect("nonexistent")
	if err == nil {
		t.Fatal("Connect() should return error for nonexistent host")
	}
}

func TestSSHManager_Disconnect_NotConnected(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// 接続していないホストの切断はエラーにならない
	if err := sm.Disconnect("server1"); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
}

func TestSSHManager_IsConnected_NotLoaded(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if sm.IsConnected("server1") {
		t.Error("should not be connected before LoadHosts")
	}
}

func TestSSHManager_GetConnection(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() SSHConnection {
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// 接続前
	_, err := sm.GetConnection("server1")
	if err == nil {
		t.Fatal("GetConnection() should return error when not connected")
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// 接続後（mock では nil client）
	client, err := sm.GetConnection("server1")
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	// mock では nil
	_ = client
}

func TestSSHManager_GetSSHConnection(t *testing.T) {
	hosts := testHosts()
	mockConn := &mockSSHConnection{client: nil, isAlive: true}
	sm := newTestSSHManager(hosts, func() SSHConnection {
		return mockConn
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	// 接続前
	_, err := sm.GetSSHConnection("server1")
	if err == nil {
		t.Fatal("GetSSHConnection() should return error when not connected")
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	conn, err := sm.GetSSHConnection("server1")
	if err != nil {
		t.Fatalf("GetSSHConnection() error = %v", err)
	}
	if conn != mockConn {
		t.Error("GetSSHConnection() returned unexpected connection")
	}
}

func TestSSHManager_ReloadHosts_PreservesState(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() SSHConnection {
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// リロード
	reloaded, err := sm.ReloadHosts()
	if err != nil {
		t.Fatalf("ReloadHosts() error = %v", err)
	}

	if len(reloaded) != 2 {
		t.Fatalf("len(hosts) = %d, want 2", len(reloaded))
	}

	// server1 の接続状態が保持されていること
	if reloaded[0].State != Connected {
		t.Errorf("server1 state = %v, want %v", reloaded[0].State, Connected)
	}
	// server2 は変わらない
	if reloaded[1].State != Disconnected {
		t.Errorf("server2 state = %v, want %v", reloaded[1].State, Disconnected)
	}
}

func TestSSHManager_Close(t *testing.T) {
	hosts := testHosts()
	mockConn := &mockSSHConnection{client: nil, isAlive: true}
	sm := newTestSSHManager(hosts, func() SSHConnection {
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

func TestSSHManager_Subscribe_MultipleSubscribers(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() SSHConnection {
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
	for _, ch := range []<-chan SSHEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != SSHEventConnected {
				t.Errorf("event type = %v, want %v", ev.Type, SSHEventConnected)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
}

func TestSSHManager_HandleDisconnect_WithReconnect(t *testing.T) {
	hosts := testHosts()
	connectCount := 0
	var mu sync.Mutex

	parser := &mockSSHConfigParser{hosts: hosts}
	sm := NewSSHManager(
		parser,
		func() SSHConnection {
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
		ReconnectConfig{
			Enabled:      true,
			MaxRetries:   3,
			InitialDelay: Duration{Duration: 10 * time.Millisecond},
			MaxDelay:     Duration{Duration: 50 * time.Millisecond},
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
	expectedTypes := []SSHEventType{
		SSHEventConnected,    // 初回接続
		SSHEventDisconnected, // 切断検出
		SSHEventReconnecting, // 再接続開始
		SSHEventConnected,    // 再接続成功
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

func TestSSHManager_Connect_ConcurrentSameHost(t *testing.T) {
	// 同一ホストへの並行 Connect が安全に動作し、接続ファクトリが1回しか呼ばれないことを確認する。
	hosts := testHosts()
	var callCount int
	var mu sync.Mutex

	sm := newTestSSHManager(hosts, func() SSHConnection {
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

func TestSSHManager_Disconnect_StopsReconnect(t *testing.T) {
	// Disconnect がホストの再接続ループを停止することを確認する。
	hosts := testHosts()
	var connectCount int
	var mu sync.Mutex

	parser := &mockSSHConfigParser{hosts: hosts}
	sm := NewSSHManager(
		parser,
		func() SSHConnection {
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
		ReconnectConfig{
			Enabled:      true,
			MaxRetries:   100, // 多めに設定
			InitialDelay: Duration{Duration: 10 * time.Millisecond},
			MaxDelay:     Duration{Duration: 50 * time.Millisecond},
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
		if ev.Type != SSHEventConnected {
			t.Fatalf("expected Connected, got %v", ev.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for connected event")
	}

	// Disconnected を待つ
	select {
	case ev := <-events:
		if ev.Type != SSHEventDisconnected {
			t.Fatalf("expected Disconnected, got %v", ev.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for disconnected event")
	}

	// Reconnecting を待つ
	select {
	case ev := <-events:
		if ev.Type != SSHEventReconnecting {
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
