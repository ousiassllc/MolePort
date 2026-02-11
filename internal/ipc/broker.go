package ipc

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ousiassllc/moleport/internal/core"
)

// Subscription はクライアントのイベント購読を表す。
type Subscription struct {
	ID       string
	ClientID string
	Types    map[string]bool // "ssh", "forward", "metrics"
}

// NotifySender はクライアントに通知を送信する関数の型。
type NotifySender func(clientID string, notification Notification) error

// EventBroker はクライアント単位のイベント購読を管理し、コアマネージャーからの通知を配信する。
type EventBroker struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription // subscriptionID -> Subscription
	clientSubs    map[string][]string      // clientID -> []subscriptionID
	sender        NotifySender
	nextID        atomic.Int64
}

// NewEventBroker は新しい EventBroker を生成する。
func NewEventBroker(sender NotifySender) *EventBroker {
	return &EventBroker{
		subscriptions: make(map[string]*Subscription),
		clientSubs:    make(map[string][]string),
		sender:        sender,
	}
}

// Subscribe はクライアントのイベント購読を登録し、購読 ID を返す。
func (b *EventBroker) Subscribe(clientID string, types []string) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID.Add(1)
	subID := fmt.Sprintf("sub-%s-%d", clientID, id)

	typeMap := make(map[string]bool, len(types))
	for _, t := range types {
		typeMap[t] = true
	}

	sub := &Subscription{
		ID:       subID,
		ClientID: clientID,
		Types:    typeMap,
	}

	b.subscriptions[subID] = sub
	b.clientSubs[clientID] = append(b.clientSubs[clientID], subID)

	return subID
}

// Unsubscribe は購読を解除する。成功すると true を返す。
func (b *EventBroker) Unsubscribe(subscriptionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub, ok := b.subscriptions[subscriptionID]
	if !ok {
		return false
	}

	delete(b.subscriptions, subscriptionID)

	// clientSubs から削除
	subs := b.clientSubs[sub.ClientID]
	for i, id := range subs {
		if id == subscriptionID {
			b.clientSubs[sub.ClientID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(b.clientSubs[sub.ClientID]) == 0 {
		delete(b.clientSubs, sub.ClientID)
	}

	return true
}

// RemoveClient はクライアントの全購読を削除する。切断時に呼ばれる。
func (b *EventBroker) RemoveClient(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subIDs := b.clientSubs[clientID]
	for _, id := range subIDs {
		delete(b.subscriptions, id)
	}
	delete(b.clientSubs, clientID)
}

// HandleSSHEvent は SSH イベントを変換し、購読者に配信する。
func (b *EventBroker) HandleSSHEvent(evt core.SSHEvent) {
	notif := SSHEventNotification{
		Type: sshEventTypeToString(evt.Type),
		Host: evt.HostName,
	}
	if evt.Error != nil {
		notif.Error = evt.Error.Error()
	}

	b.distribute("ssh", "event.ssh", notif)
}

// HandleForwardEvent はポートフォワーディングイベントを変換し、購読者に配信する。
func (b *EventBroker) HandleForwardEvent(evt core.ForwardEvent) {
	notif := ForwardEventNotification{
		Type: forwardEventTypeToString(evt.Type),
		Name: evt.RuleName,
	}
	if evt.Session != nil {
		notif.Host = evt.Session.Rule.Host
	}
	if evt.Error != nil {
		notif.Error = evt.Error.Error()
	}

	b.distribute("forward", "event.forward", notif)
}

// distribute は指定イベント種別の購読者全員に通知を送信する。
func (b *EventBroker) distribute(eventType string, method string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	notif := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  data,
	}

	// ロック中にターゲットを収集し、ロック解放後に送信する
	b.mu.RLock()
	sent := make(map[string]bool)
	var targets []string
	for _, sub := range b.subscriptions {
		if sub.Types[eventType] && !sent[sub.ClientID] {
			sent[sub.ClientID] = true
			targets = append(targets, sub.ClientID)
		}
	}
	b.mu.RUnlock()

	for _, clientID := range targets {
		go b.sender(clientID, notif)
	}
}

// sshEventTypeToString は SSHEventType をイベント通知用の文字列に変換する。
func sshEventTypeToString(t core.SSHEventType) string {
	if t == core.SSHEventPendingAuth {
		return "pending_auth"
	}
	return strings.ToLower(t.String())
}

// forwardEventTypeToString は ForwardEventType を小文字の文字列に変換する。
func forwardEventTypeToString(t core.ForwardEventType) string {
	return strings.ToLower(t.String())
}
