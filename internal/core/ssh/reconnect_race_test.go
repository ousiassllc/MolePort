package ssh

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

// Reconnecting 中の手動 Connect が優先され、後続の再接続成功が既存接続を上書きしない（接続は 1 本だけ残り、
// 再接続側で確立した接続は Close される）こと。
func TestSSHManager_ManualConnectDuringReconnect_KeepsSingleConnection(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	dropped := &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
	reconnected := &mockSSHConnection{
		isAlive: true,
		dialF: func(core.SSHHost) (*cryptossh.Client, error) {
			close(started)
			<-release
			return nil, nil
		},
	}
	manual := &mockSSHConnection{isAlive: true}
	conns := []*mockSSHConnection{dropped, reconnected, manual}
	var calls atomic.Int32
	sm := newDialTestManager(t, func() core.SSHConnection {
		return conns[calls.Add(1)-1]
	}, fastReconnectConfig(3))

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	recvWithTimeout(t, started, "reconnect Dial start")

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("manual Connect() error = %v", err)
	}
	close(release)

	deadline := time.Now().Add(dialTestTimeout)
	for !reconnected.isClosed() {
		if time.Now().After(deadline) {
			t.Fatal("connection established by reconnect must be closed when a manual connection exists")
		}
		time.Sleep(time.Millisecond)
	}
	if manual.isClosed() {
		t.Error("manual connection must not be closed")
	}
	if !sm.IsConnected("server1") {
		t.Error("server1 must stay connected through the manual connection")
	}
}

// 置き換え済み（現在の接続ではない）hostConnection の切断検出が、現在の健全な接続を切断しないこと。
func TestSSHManager_HandleDisconnect_IgnoresStaleConnection(t *testing.T) {
	current := &mockSSHConnection{isAlive: true}
	sm := newDialTestManager(t, func() core.SSHConnection { return current }, fastReconnectConfig(3))
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	sm.(*sshManager).handleDisconnect("server1", &hostConnection{})

	if current.isClosed() || !sm.IsConnected("server1") {
		t.Error("stale hostConnection's disconnect must not affect the current connection")
	}
}

// 再接続の試行が成功しても、既に別の接続が登録されている場合は確立した接続を破棄すること。
func TestSSHManager_TryReconnect_DiscardsWhenConnectionExists(t *testing.T) {
	existing := &mockSSHConnection{isAlive: true}
	redialed := &mockSSHConnection{isAlive: true}
	var calls atomic.Int32
	sm := newDialTestManager(t, func() core.SSHConnection {
		if calls.Add(1) == 1 {
			return existing
		}
		return redialed
	}, fastReconnectConfig(3))
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	host, err := sm.GetHost("server1")
	if err != nil {
		t.Fatalf("GetHost() error = %v", err)
	}

	if got := sm.(*sshManager).tryReconnect(context.Background(), "server1", *host); got != reconnectCancelled {
		t.Errorf("tryReconnect() = %v, want reconnectCancelled", got)
	}

	if !redialed.isClosed() {
		t.Error("redialed connection must be closed")
	}
	if existing.isClosed() || !sm.IsConnected("server1") {
		t.Error("existing connection must be kept")
	}
}
