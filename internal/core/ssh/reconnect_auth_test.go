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

func TestSSHManager_Reconnect_AuthFailureStopsRetryAndPendsAuth(t *testing.T) {
	var factoryCalls atomic.Int32
	factory := func() core.SSHConnection {
		if factoryCalls.Add(1) == 1 {
			return &mockSSHConnection{isAlive: true, keepAliveF: func(context.Context, time.Duration) {}}
		}
		return &mockSSHConnection{dialF: func(core.SSHHost) (*cryptossh.Client, error) {
			return nil, fmt.Errorf("ssh: handshake failed: ssh: unable to authenticate")
		}}
	}
	sm := newDialTestManager(t, factory, core.ReconnectConfig{
		Enabled:      true,
		MaxRetries:   5,
		InitialDelay: core.Duration{Duration: time.Millisecond},
		MaxDelay:     core.Duration{Duration: time.Millisecond},
	})
	events := sm.Subscribe()
	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	for i, want := range []core.SSHEventType{
		core.SSHEventConnected, core.SSHEventDisconnected, core.SSHEventReconnecting, core.SSHEventPendingAuth,
	} {
		if ev := recvWithTimeout(t, events, fmt.Sprintf("event[%d]", i)); ev.Type != want {
			t.Fatalf("event[%d] type = %v, want %v", i, ev.Type, want)
		}
	}
	// PendingAuth は再接続ループの最後に発行されるため、試行回数はここで確定している
	if got := factoryCalls.Load(); got != 2 {
		t.Errorf("connFactory called %d times, want 2 (initial connect + one reconnect attempt)", got)
	}
	if host, _ := sm.GetHost("server1"); host.State != core.PendingAuth {
		t.Errorf("host state = %v, want %v", host.State, core.PendingAuth)
	}
}
