package daemon

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestStartEventRouting_ReconnectAborted(t *testing.T) {
	t.Run("reconnecting_then_pending_auth_fails_forwards", func(t *testing.T) {
		sshCh, fwdCh := make(chan core.SSHEvent, 4), make(chan core.ForwardEvent, 1)
		fwd := &mockForwardManagerForState{subscribeCh: fwdCh}
		d := &Daemon{sshMgr: &mockSSHManagerForState{subscribeCh: sshCh}, fwdMgr: fwd, broker: newBrokerStub()}
		d.startEventRouting()
		sshCh <- core.SSHEvent{Type: core.SSHEventReconnecting, HostName: "h1"}
		sshCh <- core.SSHEvent{Type: core.SSHEventPendingAuth, HostName: "h1"}
		close(sshCh)
		close(fwdCh)
		d.wg.Wait()
		fwd.mu.Lock()
		defer fwd.mu.Unlock()
		if len(fwd.failReconnectingCalls) != 1 || fwd.failReconnectingCalls[0] != "h1" {
			t.Errorf("failReconnecting = %v, want [h1]", fwd.failReconnectingCalls)
		}
	})
	t.Run("disconnect_while_reconnecting_does_not_restore_on_later_connect", func(t *testing.T) {
		sshCh, fwdCh := make(chan core.SSHEvent, 4), make(chan core.ForwardEvent, 1)
		restored := false
		fwd := &mockForwardManagerForState{subscribeCh: fwdCh, restoreForwardsFn: func(string) []core.ForwardRestoreResult {
			restored = true
			return nil
		}}
		d := &Daemon{sshMgr: &mockSSHManagerForState{subscribeCh: sshCh}, fwdMgr: fwd, broker: newBrokerStub()}
		d.startEventRouting()
		sshCh <- core.SSHEvent{Type: core.SSHEventReconnecting, HostName: "h1"}
		sshCh <- core.SSHEvent{Type: core.SSHEventDisconnected, HostName: "h1"}
		sshCh <- core.SSHEvent{Type: core.SSHEventConnected, HostName: "h1"}
		close(sshCh)
		close(fwdCh)
		d.wg.Wait()
		if restored {
			t.Error("RestoreForwards called after the user disconnected during reconnect")
		}
	})
}
