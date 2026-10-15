package tray

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_CoalescesTriggers(t *testing.T) {
	var calls atomic.Int32
	done := make(chan struct{}, 4)
	d := NewDebouncer(20*time.Millisecond, func() { calls.Add(1); done <- struct{}{} })
	defer d.Stop()

	for i := 0; i < 10; i++ {
		d.Trigger()
	}
	<-done
	time.Sleep(60 * time.Millisecond)

	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestDebouncer_TriggerAfterFireSchedulesAgain(t *testing.T) {
	done := make(chan struct{}, 4)
	d := NewDebouncer(10*time.Millisecond, func() { done <- struct{}{} })
	defer d.Stop()

	for i := 0; i < 2; i++ {
		d.Trigger()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("%d 回目の Trigger が実行されなかった", i+1)
		}
	}
}

func TestDebouncer_StopCancelsPending(t *testing.T) {
	var calls atomic.Int32
	d := NewDebouncer(20*time.Millisecond, func() { calls.Add(1) })

	d.Trigger()
	d.Stop()
	d.Trigger()
	time.Sleep(60 * time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Errorf("Stop 後に fn が %d 回呼ばれた", got)
	}
}
