package ipc

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

// collectingSender はテスト用に通知を収集する NotifySender を返す。
func collectingSender() (NotifySender, *notifLog) {
	log := &notifLog{}
	sender := func(clientID string, notification Notification) error {
		log.mu.Lock()
		defer log.mu.Unlock()
		log.entries = append(log.entries, notifEntry{
			ClientID:     clientID,
			Notification: notification,
		})
		return nil
	}
	return sender, log
}

type notifEntry struct {
	ClientID     string
	Notification Notification
}

type notifLog struct {
	mu      sync.Mutex
	entries []notifEntry
}

func (l *notifLog) get() []notifEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]notifEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func TestEventBroker_Subscribe(t *testing.T) {
	sender, _ := collectingSender()
	broker := NewEventBroker(sender)

	subID := broker.Subscribe("client-1", []string{"ssh", "forward"})
	if subID == "" {
		t.Fatal("Subscribe should return non-empty subscription ID")
	}

	// 購読が登録されていることを確認
	broker.mu.RLock()
	sub, ok := broker.subscriptions[subID]
	broker.mu.RUnlock()

	if !ok {
		t.Fatal("subscription should exist")
	}
	if sub.ClientID != "client-1" {
		t.Errorf("ClientID = %q, want %q", sub.ClientID, "client-1")
	}
	if !sub.Types["ssh"] || !sub.Types["forward"] {
		t.Errorf("Types = %v, want ssh and forward", sub.Types)
	}
}

func TestEventBroker_Unsubscribe(t *testing.T) {
	sender, _ := collectingSender()
	broker := NewEventBroker(sender)

	subID := broker.Subscribe("client-1", []string{"ssh"})

	if !broker.Unsubscribe(subID) {
		t.Fatal("Unsubscribe should return true for existing subscription")
	}

	// 二重解除は false を返す
	if broker.Unsubscribe(subID) {
		t.Fatal("Unsubscribe should return false for already removed subscription")
	}

	// 存在しない ID も false
	if broker.Unsubscribe("nonexistent") {
		t.Fatal("Unsubscribe should return false for nonexistent subscription")
	}

	// 内部状態が空であることを確認
	broker.mu.RLock()
	defer broker.mu.RUnlock()
	if len(broker.subscriptions) != 0 {
		t.Errorf("subscriptions should be empty, got %d", len(broker.subscriptions))
	}
	if len(broker.clientSubs) != 0 {
		t.Errorf("clientSubs should be empty, got %d", len(broker.clientSubs))
	}
}

func TestEventBroker_RemoveClient(t *testing.T) {
	sender, _ := collectingSender()
	broker := NewEventBroker(sender)

	broker.Subscribe("client-1", []string{"ssh"})
	broker.Subscribe("client-1", []string{"forward"})
	broker.Subscribe("client-2", []string{"ssh"})

	broker.RemoveClient("client-1")

	broker.mu.RLock()
	defer broker.mu.RUnlock()

	// client-1 の購読がすべて削除されていること
	if _, ok := broker.clientSubs["client-1"]; ok {
		t.Error("client-1 subscriptions should be removed")
	}

	// client-2 の購読は残っていること
	if len(broker.clientSubs["client-2"]) != 1 {
		t.Errorf("client-2 should have 1 subscription, got %d", len(broker.clientSubs["client-2"]))
	}

	// 全体の購読数が 1 であること
	if len(broker.subscriptions) != 1 {
		t.Errorf("total subscriptions should be 1, got %d", len(broker.subscriptions))
	}
}

func TestEventBroker_HandleSSHEvent(t *testing.T) {
	sender, log := collectingSender()
	broker := NewEventBroker(sender)

	// SSH を購読するクライアント
	broker.Subscribe("client-ssh", []string{"ssh"})
	// forward のみ購読するクライアント
	broker.Subscribe("client-fwd", []string{"forward"})

	evt := core.SSHEvent{
		Type:     core.SSHEventConnected,
		HostName: "prod-server",
	}

	broker.HandleSSHEvent(evt)

	// goroutine で送信されるため少し待つ
	waitForEntries(t, log, 1)

	entries := log.get()
	if len(entries) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(entries))
	}

	if entries[0].ClientID != "client-ssh" {
		t.Errorf("notification should go to client-ssh, got %q", entries[0].ClientID)
	}

	if entries[0].Notification.Method != "event.ssh" {
		t.Errorf("method = %q, want %q", entries[0].Notification.Method, "event.ssh")
	}

	var notif SSHEventNotification
	if err := json.Unmarshal(entries[0].Notification.Params, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if notif.Type != "connected" {
		t.Errorf("event type = %q, want %q", notif.Type, "connected")
	}
	if notif.Host != "prod-server" {
		t.Errorf("event host = %q, want %q", notif.Host, "prod-server")
	}
}

func TestEventBroker_HandleSSHEvent_WithError(t *testing.T) {
	sender, log := collectingSender()
	broker := NewEventBroker(sender)

	broker.Subscribe("client-1", []string{"ssh"})

	evt := core.SSHEvent{
		Type:     core.SSHEventError,
		HostName: "prod-server",
		Error:    errors.New("connection refused"),
	}

	broker.HandleSSHEvent(evt)

	waitForEntries(t, log, 1)

	entries := log.get()
	var notif SSHEventNotification
	if err := json.Unmarshal(entries[0].Notification.Params, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if notif.Error != "connection refused" {
		t.Errorf("error = %q, want %q", notif.Error, "connection refused")
	}
}

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

	if entries[0].Notification.Method != "event.forward" {
		t.Errorf("method = %q, want %q", entries[0].Notification.Method, "event.forward")
	}

	var notif ForwardEventNotification
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
			clientID := "client-" + string(rune('A'+n))

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

// waitForEntries は通知ログに指定数のエントリが蓄積されるまで待つ。
func waitForEntries(t *testing.T, log *notifLog, count int) {
	t.Helper()
	waitFor(t, func() bool {
		return len(log.get()) >= count
	})
}
