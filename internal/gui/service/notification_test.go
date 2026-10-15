package service

import (
	"fmt"
	"sync"
	"testing"
)

func TestNotificationQueue_PushDrainFIFO(t *testing.T) {
	q := NewNotificationQueue()
	q.Push(ToastView{Kind: ToastInfo, Message: "1"})
	q.Push(ToastView{Kind: ToastError, Message: "2"})

	got := q.Drain()
	if len(got) != 2 || got[0].Message != "1" || got[1].Message != "2" {
		t.Fatalf("Drain() = %+v, want [1 2]", got)
	}
	if again := q.Drain(); len(again) != 0 {
		t.Errorf("2 回目の Drain() = %+v, want 空", again)
	}
}

func TestNotificationQueue_DropsOldestBeyondLimit(t *testing.T) {
	q := NewNotificationQueue()
	for i := range MaxNotifications + 5 {
		q.Push(ToastView{Message: fmt.Sprint(i)})
	}

	got := q.Drain()
	if len(got) != MaxNotifications {
		t.Fatalf("len = %d, want %d", len(got), MaxNotifications)
	}
	if got[0].Message != "5" || got[len(got)-1].Message != fmt.Sprint(MaxNotifications+4) {
		t.Errorf("先頭 = %q, 末尾 = %q（古いものから破棄される）", got[0].Message, got[len(got)-1].Message)
	}
}

func TestNotificationQueue_ConcurrentPush(t *testing.T) {
	q := NewNotificationQueue()
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 10 {
				q.Push(ToastView{Message: "x"})
			}
		})
	}
	wg.Wait()
	if got := len(q.Drain()); got != 50 {
		t.Errorf("len = %d, want 50", got)
	}
}
