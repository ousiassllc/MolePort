package ipc

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestEventBroker_HandleForwardEvent(t *testing.T) {
	sender, log := collectingSender()
	broker := NewEventBroker(sender)

	// forward を購読するクライアント
	broker.Subscribe("client-fwd", []string{"forward"})
	// SSH のみ購読するクライアント
	broker.Subscribe("client-ssh", []string{"ssh"})

	evt := core.ForwardEvent{
		Type:     core.ForwardEventStarted,
		RuleName: "web-proxy",
		Session: &core.ForwardSession{
			Rule: core.ForwardRule{Host: "prod-server"},
		},
	}

	broker.HandleForwardEvent(evt)

	waitForEntries(t, log, 1)

	entries := log.get()
	if len(entries) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(entries))
	}

	if entries[0].ClientID != "client-fwd" {
		t.Errorf("notification should go to client-fwd, got %q", entries[0].ClientID)
	}

	if entries[0].Notification.Method != protocol.EventForward {
		t.Errorf("method = %q, want %q", entries[0].Notification.Method, protocol.EventForward)
	}

	var notif protocol.ForwardEventNotification
	if err := json.Unmarshal(entries[0].Notification.Params, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if notif.Type != "started" {
		t.Errorf("event type = %q, want %q", notif.Type, "started")
	}
	if notif.Name != "web-proxy" {
		t.Errorf("event name = %q, want %q", notif.Name, "web-proxy")
	}
	if notif.Host != "prod-server" {
		t.Errorf("event host = %q, want %q", notif.Host, "prod-server")
	}
}

func TestEventBroker_MultipleClients(t *testing.T) {
	sender, log := collectingSender()
	broker := NewEventBroker(sender)

	// 3 クライアントが SSH を購読
	broker.Subscribe("client-1", []string{"ssh"})
	broker.Subscribe("client-2", []string{"ssh"})
	broker.Subscribe("client-3", []string{"ssh"})

	evt := core.SSHEvent{
		Type:     core.SSHEventConnected,
		HostName: "prod",
	}

	broker.HandleSSHEvent(evt)

	waitForEntries(t, log, 3)

	entries := log.get()
	if len(entries) != 3 {
		t.Fatalf("expected 3 notifications, got %d", len(entries))
	}

	// 各クライアントに通知が届いていること
	clients := make(map[string]bool)
	for _, e := range entries {
		clients[e.ClientID] = true
	}
	for _, id := range []string{"client-1", "client-2", "client-3"} {
		if !clients[id] {
			t.Errorf("client %s did not receive notification", id)
		}
	}
}

func TestEventBroker_ConcurrentAccess(t *testing.T) {
	sender, _ := collectingSender()
	broker := NewEventBroker(sender)

	var wg sync.WaitGroup
	var ops atomic.Int64

	// 並行して購読・解除・イベント送信を実行
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			clientID := "client-" + string(rune('A'+n)) //nolint:gosec // n is always 0-9

			subID := broker.Subscribe(clientID, []string{"ssh", "forward"})
			ops.Add(1)

			broker.HandleSSHEvent(core.SSHEvent{
				Type:     core.SSHEventConnected,
				HostName: "host",
			})
			ops.Add(1)

			broker.Unsubscribe(subID)
			ops.Add(1)
		}(i)
	}

	wg.Wait()

	if ops.Load() != 30 {
		t.Errorf("expected 30 operations, got %d", ops.Load())
	}
}
