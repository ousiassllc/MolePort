package core

import (
	"fmt"
	"log/slog"
	"sync"
)

// EventChannelBuffer はイベントチャネルのバッファサイズ。
const EventChannelBuffer = 16

// EventEmitter はイベントの配信を管理するジェネリック型。
// mu は埋め込み先の *sync.RWMutex をポインタで共有する。
type EventEmitter[E any] struct {
	mu          *sync.RWMutex
	subscribers []chan E
}

// NewEventEmitter は EventEmitter を初期化して返す。
func NewEventEmitter[E any](mu *sync.RWMutex) EventEmitter[E] {
	return EventEmitter[E]{mu: mu}
}

// Emit はイベントを全サブスクライバーに非ブロッキングで送信する。
func (e *EventEmitter[E]) Emit(event E) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, ch := range e.subscribers {
		select {
		case ch <- event:
		default:
			slog.Warn("event dropped", "event_type", fmt.Sprintf("%T", event))
		}
	}
}

// Subscribe はイベントチャネルを作成・登録して返す。
// 呼び出し元が mu.Lock() を保持していること。
func (e *EventEmitter[E]) Subscribe() chan E {
	ch := make(chan E, EventChannelBuffer)
	e.subscribers = append(e.subscribers, ch)
	return ch
}

// CloseSubscribers は全チャネルをクローズし、サブスクライバー一覧をクリアする。
// 呼び出し元が mu.Lock() を保持していること。
func (e *EventEmitter[E]) CloseSubscribers() {
	for _, ch := range e.subscribers {
		close(ch)
	}
	e.subscribers = nil
}
