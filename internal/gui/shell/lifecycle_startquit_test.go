package shell

import (
	"context"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/gui/service"
)

// 起動中（host.Start 中）に終了が要求されても、デーモンを停止して ready にしない。
// Stop は Start 完了後に 1 回だけ呼ばれ、終了要求は停止の完了まで返らない。
func TestLifecycle_QuitDuringDaemonStartStopsDaemon(t *testing.T) {
	tests := []struct {
		name string
		quit func(l *Lifecycle)
	}{
		{"RequestQuit（トレイ・シグナル・ウィンドウ）", func(l *Lifecycle) { l.RequestQuit(false) }},
		{"Shutdown（OnShutdown）", func(l *Lifecycle) { l.Shutdown() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newFakeDaemon()
			d.startEntered, d.startGate = make(chan struct{}), make(chan struct{})
			h := newHarness(t, d)

			startDone := make(chan error, 1)
			go func() { startDone <- h.life.Start(context.Background()) }()
			<-d.startEntered

			quitDone := make(chan struct{})
			go func() {
				defer close(quitDone)
				tt.quit(h.life)
			}()
			// Start が終わるまで終了要求は完了しない（Stop は Start 後でなければならない）。
			select {
			case <-quitDone:
				t.Fatal("起動完了前に終了要求が返った")
			case <-time.After(50 * time.Millisecond):
			}
			close(d.startGate)

			if err := <-startDone; err != nil {
				t.Fatalf("Start: %v", err)
			}
			<-quitDone

			if got := d.stopCount(); got != 1 {
				t.Errorf("Stop 回数 = %d, want 1", got)
			}
			if got := h.life.State(); got == service.LifecycleReady {
				t.Error("終了要求後に ready にしてはならない")
			}
			if _, ok := h.life.Backend(); ok {
				t.Error("終了要求後に Backend が返ってはならない")
			}
			if len(h.ready) != 0 {
				t.Error("終了要求後に OnReady を呼んではならない")
			}
			if len(h.notifier.got) != 0 {
				t.Errorf("終了要求後に通知してはならない: %v", h.notifier.got)
			}
		})
	}
}

// 引き継ぎ中に終了が要求されたら、デーモンを起動しない。
func TestLifecycle_QuitDuringTakeoverDoesNotStartDaemon(t *testing.T) {
	d := newFakeDaemon()
	h := newHarness(t, d)
	h.takeHook = func() { h.life.RequestQuit(false) }

	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	h.waitQuit(t)
	if d.startCount() != 0 {
		t.Error("終了要求後にデーモンを起動した")
	}
	if got := h.life.State(); got == service.LifecycleReady {
		t.Error("終了要求後に ready にしてはならない")
	}
}
