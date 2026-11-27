package forwardint_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/forward"
	"github.com/ousiassllc/moleport/internal/core/forwardtest"
	"github.com/ousiassllc/moleport/internal/ipc"
	"github.com/ousiassllc/moleport/internal/ipc/handler"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// newRealForwardHandler は実 ForwardManager（モック SSH 上）を使う Handler を返す。
// prod / staging は接続済みで、dynamic ルールを rules として追加済み。
func newRealForwardHandler(t *testing.T, rules ...core.ForwardRule) (*handler.Handler, core.ForwardManager) {
	t.Helper()
	sm := forwardtest.NewMockSSHManager()
	for _, host := range []string{"prod", "staging"} {
		sm.SetConnected(host, forwardtest.NewMockConn(false, true))
	}
	fm := forward.NewForwardManager(context.Background(), sm)
	t.Cleanup(fm.Close)
	for _, r := range rules {
		r.Type = core.Dynamic
		if _, err := fm.AddRule(r); err != nil {
			t.Fatalf("AddRule(%q) error = %v", r.Name, err)
		}
	}
	broker := ipc.NewEventBroker(func(_ string, _ protocol.Notification) error { return nil })
	return handler.NewHandler(sm, fm, nil, broker, nil, nil), fm
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return b
}

// ssh.disconnect は当該ホストの転送を停止し、他ホストの転送には影響しない。
func TestHandler_SSHDisconnect_StopsHostForwards(t *testing.T) {
	h, fm := newRealForwardHandler(t,
		core.ForwardRule{Name: "prod-1", Host: "prod", LocalPort: 1081},
		core.ForwardRule{Name: "prod-2", Host: "prod", LocalPort: 1082},
		core.ForwardRule{Name: "staging-1", Host: "staging", LocalPort: 1083},
	)
	for _, name := range []string{"prod-1", "prod-2", "staging-1"} {
		if err := fm.StartForward(name, nil); err != nil {
			t.Fatalf("StartForward(%q) error = %v", name, err)
		}
	}

	params := mustMarshal(t, protocol.SSHDisconnectParams{Host: "prod"})
	if _, rpcErr := h.Handle("client-1", "ssh.disconnect", params); rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	forwardtest.AssertSessionStatus(t, fm, "prod-1", core.Stopped)
	forwardtest.AssertSessionStatus(t, fm, "prod-2", core.Stopped)
	forwardtest.AssertSessionStatus(t, fm, "staging-1", core.Active)
}

// Error 状態（再接続失敗）の auto_connect 転送は、ssh.connect で再開される。
func TestHandler_SSHConnect_RestartsErrorAutoConnectForward(t *testing.T) {
	h, fm := newRealForwardHandler(t, core.ForwardRule{Name: "auto", Host: "prod", LocalPort: 1081, AutoConnect: true})
	if err := fm.StartForward("auto", nil); err != nil {
		t.Fatalf("StartForward() error = %v", err)
	}
	fm.MarkReconnecting("prod")
	fm.FailReconnecting("prod")
	forwardtest.AssertSessionStatus(t, fm, "auto", core.SessionError)

	params := mustMarshal(t, protocol.SSHConnectParams{Host: "prod"})
	if _, rpcErr := h.Handle("client-1", "ssh.connect", params); rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	forwardtest.AssertSessionStatus(t, fm, "auto", core.Active)
}
