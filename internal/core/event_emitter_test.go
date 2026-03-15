package core_test

import (
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestEventEmitter_MultipleSubscribers(t *testing.T) {
	var mu sync.RWMutex
	emitter := core.NewEventEmitter[string](&mu)

	mu.Lock()
	ch1 := emitter.Subscribe()
	ch2 := emitter.Subscribe()
	mu.Unlock()

	emitter.Emit("hello")

	select {
	case got := <-ch1:
		if got != "hello" {
			t.Errorf("ch1: got %q, want %q", got, "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("ch1: timed out")
	}

	select {
	case got := <-ch2:
		if got != "hello" {
			t.Errorf("ch2: got %q, want %q", got, "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("ch2: timed out")
	}
}

func TestEventEmitter_BufferFullDrop(t *testing.T) {
	var mu sync.RWMutex
	emitter := core.NewEventEmitter[int](&mu)

	mu.Lock()
	ch := emitter.Subscribe()
	mu.Unlock()

	// バッファを埋める
	for i := range core.EventChannelBuffer {
		emitter.Emit(i)
	}

	// バッファフル時にブロッキングしないことを確認
	done := make(chan struct{})
	go func() {
		emitter.Emit(999)
		close(done)
	}()

	select {
	case <-done:
		// ブロッキングせずに完了
	case <-time.After(time.Second):
		t.Fatal("Emit blocked on full channel")
	}

	// ドロップされたイベントはチャネルに入っていないことを確認
	received := 0
	for range core.EventChannelBuffer {
		select {
		case <-ch:
			received++
		default:
		}
	}

	if received != core.EventChannelBuffer {
		t.Errorf("received %d events, want %d", received, core.EventChannelBuffer)
	}

	// 追加イベントは入っていない
	select {
	case <-ch:
		t.Error("unexpected extra event in channel")
	default:
	}
}

func TestEventEmitter_CloseSubscribers(t *testing.T) {
	var mu sync.RWMutex
	emitter := core.NewEventEmitter[string](&mu)

	mu.Lock()
	ch1 := emitter.Subscribe()
	ch2 := emitter.Subscribe()
	mu.Unlock()

	mu.Lock()
	emitter.CloseSubscribers()
	mu.Unlock()

	// チャネルがクローズされていることを確認
	_, ok1 := <-ch1
	if ok1 {
		t.Error("ch1 should be closed")
	}

	_, ok2 := <-ch2
	if ok2 {
		t.Error("ch2 should be closed")
	}
}
