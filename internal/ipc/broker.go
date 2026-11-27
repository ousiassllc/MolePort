package ipc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// Subscription はクライアントのイベント購読を表す。
type Subscription struct {
	ID       string
	ClientID string
	Types    map[string]bool // "ssh", "forward", "metrics"
}

// NotifySender はクライアントに通知を送信する関数の型。
type NotifySender func(clientID string, notification protocol.Notification) error

// clientQueueSize はクライアントごとの送信キューの上限。
// これを超えて溜まるクライアントは読み取りが滞っているとみなして切断する。
const clientQueueSize = 1024

// EventBroker はクライアント単位のイベント購読を管理し、コアマネージャーからの通知を配信する。
//
// 通知はクライアントごとの有界キューと専用の送信 goroutine を介して送られるため、
// 同一クライアントへは発行順に届く。
type EventBroker struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription // subscriptionID -> Subscription
	clientSubs    map[string][]string      // clientID -> []subscriptionID
	queues        map[string]chan protocol.Notification
	sender        NotifySender
	onOverflow    func(clientID string)
	nextID        atomic.Int64
}

// NewEventBroker は新しい EventBroker を生成する。
func NewEventBroker(sender NotifySender) *EventBroker {
	return &EventBroker{
		subscriptions: make(map[string]*Subscription),
		clientSubs:    make(map[string][]string),
		queues:        make(map[string]chan protocol.Notification),
		sender:        sender,
	}
}

// SetOnOverflow は送信キューが溢れたクライアントを切断するためのコールバックを設定する。
// Subscribe の前に呼ぶこと。
func (b *EventBroker) SetOnOverflow(fn func(clientID string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onOverflow = fn
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

	if _, ok := b.queues[clientID]; !ok {
		q := make(chan protocol.Notification, clientQueueSize)
		b.queues[clientID] = q
		go b.writeLoop(clientID, q)
	}

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
		b.closeQueueLocked(sub.ClientID)
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
	b.closeQueueLocked(clientID)
}

// closeQueueLocked はクライアントの送信キューを閉じる。b.mu をロックして呼ぶこと。
func (b *EventBroker) closeQueueLocked(clientID string) {
	if q, ok := b.queues[clientID]; ok {
		close(q)
		delete(b.queues, clientID)
	}
}

// writeLoop はクライアントのキューから通知を取り出し、発行順に送信する。
func (b *EventBroker) writeLoop(clientID string, q <-chan protocol.Notification) {
	for notif := range q {
		if err := b.sender(clientID, notif); err != nil {
			slog.Debug("failed to send notification", "client", clientID, "error", err)
		}
	}
}

// HandleSSHEvent は SSH イベントを変換し、購読者に配信する。
func (b *EventBroker) HandleSSHEvent(evt core.SSHEvent) {
	notif := protocol.SSHEventNotification{
		Type: sshEventTypeToString(evt.Type),
		Host: evt.HostName,
	}
	if evt.Error != nil {
		notif.Error = evt.Error.Error()
	}

	b.distribute("ssh", protocol.EventSSH, notif)
}

// HandleForwardEvent はポートフォワーディングイベントを変換し、購読者に配信する。
func (b *EventBroker) HandleForwardEvent(evt core.ForwardEvent) {
	notif := protocol.ForwardEventNotification{
		Type: forwardEventTypeToString(evt.Type),
		Name: evt.RuleName,
	}
	if evt.Session != nil {
		notif.Host = evt.Session.Rule.Host
	}
	if evt.Error != nil {
		notif.Error = evt.Error.Error()
	}

	b.distribute("forward", protocol.EventForward, notif)
}

// distribute は指定イベント種別の購読者全員の送信キューに通知を積む。
func (b *EventBroker) distribute(eventType string, method string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("failed to marshal notification", "error", err)
		return
	}

	notif := protocol.Notification{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  method,
		Params:  data,
	}

	// ロック中に各クライアントのキューへ非ブロッキングで積む。
	// 満杯のクライアントは読み取りが滞っているため、ロック解放後に切断する。
	var overflowed []string
	b.mu.RLock()
	sent := make(map[string]bool)
	for _, sub := range b.subscriptions {
		if !sub.Types[eventType] || sent[sub.ClientID] {
			continue
		}
		sent[sub.ClientID] = true
		select {
		case b.queues[sub.ClientID] <- notif:
		default:
			overflowed = append(overflowed, sub.ClientID)
		}
	}
	onOverflow := b.onOverflow
	b.mu.RUnlock()

	for _, clientID := range overflowed {
		slog.Warn("client notification queue is full; disconnecting", "client", clientID)
		b.RemoveClient(clientID)
		if onOverflow != nil {
			onOverflow(clientID)
		}
	}
}

// sshEventTypeToString は SSHEventType をイベント通知用のワイヤー文字列に変換する。
func sshEventTypeToString(t core.SSHEventType) string {
	switch t {
	case core.SSHEventConnected:
		return protocol.StateConnected
	case core.SSHEventDisconnected:
		return protocol.StateDisconnected
	case core.SSHEventReconnecting:
		return protocol.StateReconnecting
	case core.SSHEventPendingAuth:
		return protocol.StatePendingAuth
	case core.SSHEventError:
		return protocol.StateError
	default:
		return protocol.StateDisconnected
	}
}

// forwardEventTypeToString は ForwardEventType をワイヤー文字列に変換する。
func forwardEventTypeToString(t core.ForwardEventType) string {
	switch t {
	case core.ForwardEventStarted:
		return protocol.ForwardEventTypeStarted
	case core.ForwardEventStopped:
		return protocol.ForwardEventTypeStopped
	case core.ForwardEventError:
		return protocol.ForwardEventTypeError
	case core.ForwardEventMetricsUpdated:
		return protocol.ForwardEventTypeMetricsUpdated
	case core.ForwardEventReconnecting:
		return protocol.ForwardEventTypeReconnecting
	case core.ForwardEventRestored:
		return protocol.ForwardEventTypeRestored
	default:
		return "unknown"
	}
}
