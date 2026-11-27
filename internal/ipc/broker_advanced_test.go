package ipc

import (
	"encoding/json"
	"fmt"
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

// 連続して発行したイベントは、発行順のままクライアントに届く。
func TestEventBroker_PreservesOrderPerClient(t *testing.T) {
	sender, log := collectingSender()
	broker := NewEventBroker(sender)
	broker.Subscribe("client-1", []string{"ssh"})
	broker.Subscribe("client-2", []string{"ssh"})

	const n = 500
	for i := range n {
		broker.HandleSSHEvent(core.SSHEvent{Type: core.SSHEventConnected, HostName: fmt.Sprintf("host-%d", i)})
	}
	waitForEntries(t, log, 2*n)

	next := map[string]int{}
	for _, e := range log.get() {
		var notif protocol.SSHEventNotification
		if err := json.Unmarshal(e.Notification.Params, &notif); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		want := fmt.Sprintf("host-%d", next[e.ClientID])
		if notif.Host != want {
			t.Fatalf("%s: got %s, want %s (out of order)", e.ClientID, notif.Host, want)
		}
		next[e.ClientID]++
	}
}

// 送信が止まったクライアントがいても他クライアントへ配信され、
// 溢れたクライアントは購読解除のうえ切断コールバックが呼ばれる。
func TestEventBroker_SlowClientIsDisconnectedWithoutBlockingOthers(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var fastCount atomic.Int64
	sender := func(clientID string, _ protocol.Notification) error {
		if clientID == "slow" {
			<-release
			return nil
		}
		fastCount.Add(1)
		return nil
	}
	broker := NewEventBroker(sender)
	overflow := make(chan string, 4)
	broker.SetOnOverflow(func(clientID string) { overflow <- clientID })
	broker.Subscribe("slow", []string{"ssh"})
	broker.Subscribe("fast", []string{"ssh"})

	total := clientQueueSize + 10
	for range total {
		broker.HandleSSHEvent(core.SSHEvent{Type: core.SSHEventConnected, HostName: "h"})
	}

	select {
	case id := <-overflow:
		if id != "slow" {
			t.Fatalf("overflow client = %q, want slow", id)
		}
	default:
		t.Fatal("expected overflow callback for the slow client")
	}
	waitFor(t, func() bool { return fastCount.Load() == int64(total) })

	broker.mu.RLock()
	defer broker.mu.RUnlock()
	if _, ok := broker.clientSubs["slow"]; ok {
		t.Error("slow client subscriptions should be removed")
	}
	if _, ok := broker.queues["slow"]; ok {
		t.Error("slow client queue should be removed")
	}
}
