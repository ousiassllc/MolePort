package shell

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/gui/service"
)

func TestLifecycle_TakeoverToastMentionsPIDAndAutoRestore(t *testing.T) {
	tests := []struct {
		name        string
		autoRestore bool
		wantKey     string
	}{
		{"自動復元あり", true, "gui.toast.takeover"},
		{"自動復元なし", false, "gui.toast.takeover_no_restore"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &fakeConfigManager{cfg: core.Config{Session: core.SessionConfig{AutoRestore: tt.autoRestore}}}
			d := newFakeDaemon()
			d.cfg = cfg
			h := newHarness(t, d)
			h.takePID = 4321

			if err := h.life.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}

			toasts := h.queue.Drain()
			if len(toasts) != 1 {
				t.Fatalf("toasts = %+v, want 1 件", toasts)
			}
			want := takeoverMessage(4321, service.Backend{Config: cfg})
			if toasts[0].Message != want || !strings.Contains(want, "4321") {
				t.Errorf("Message = %q, want PID を含む %q", toasts[0].Message, want)
			}
			if tt.autoRestore == strings.Contains(want, "auto_restore") {
				t.Errorf("auto_restore の併記が不正: %q", want)
			}
		})
	}
}

func TestLifecycle_RunShowsErrorDialogAndQuits(t *testing.T) {
	h := newHarness(t, newFakeDaemon())
	h.takeErr = daemon.ErrTakeoverTimeout

	h.life.Run(context.Background())

	if got := h.dialog.errKeys; len(got) != 1 || got[0] != "gui.dialog.takeover_timeout" {
		t.Errorf("ShowError = %v, want [gui.dialog.takeover_timeout]", got)
	}
	h.waitQuit(t)
	if h.daemon.stopCount() != 0 {
		t.Error("起動失敗時に Stop を呼んだ")
	}
}

func TestLifecycle_RunSuccessShowsNoDialog(t *testing.T) {
	h := newHarness(t, newFakeDaemon())

	h.life.Run(context.Background())

	if len(h.dialog.errKeys) != 0 {
		t.Errorf("成功時にエラーダイアログ: %v", h.dialog.errKeys)
	}
	h.noQuit(t)
}

// 終了要求・OnShutdown・Done 監視が同時に走っても、デーモンの Stop は 1 回で Quit も 1 回。
func TestLifecycle_ConcurrentQuitPathsStopOnce(t *testing.T) {
	d := newFakeDaemon()
	h := newHarness(t, d)
	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); h.life.RequestQuit(false) }()
		go func() { defer wg.Done(); h.life.Shutdown() }()
		go func() { defer wg.Done(); d.closeDone() }()
	}
	wg.Wait()

	// Shutdown が先に quitting を立てた場合は Wails がすでに終了中のため Quit は 0 回。重複はしない。
	time.Sleep(50 * time.Millisecond)
	if got := len(h.quits); got > 1 {
		t.Errorf("Quit 回数 = %d, want 1 以下", got)
	}
	if got := d.stopCount(); got != 1 {
		t.Errorf("Stop 回数 = %d, want 1", got)
	}
}

// 確認ダイアログの表示中に再度「終了」が要求されても、ダイアログを重ねず、最初の応答だけで決まる。
func TestLifecycle_QuitConfirmIsNotReentrant(t *testing.T) {
	d := newFakeDaemon(core.ForwardSession{Status: core.Active})
	h := newHarness(t, d)
	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	inDialog, release := make(chan struct{}), make(chan struct{})
	h.dialog.onQuitAsk = func() { close(inDialog); <-release }
	h.dialog.confirm = true

	first := make(chan struct{})
	go func() { h.life.RequestQuit(true); close(first) }()
	<-inDialog
	h.life.RequestQuit(true) // 確認中の 2 回目は何もしない

	if len(h.dialog.quitAsks) != 1 {
		t.Errorf("ダイアログが重なった: %v", h.dialog.quitAsks)
	}
	h.noQuit(t)
	close(release)
	<-first
	h.waitQuit(t)
	if d.stopCount() != 1 {
		t.Errorf("Stop 回数 = %d, want 1", d.stopCount())
	}
}

func TestLifecycle_QuitConfirmSkippedWhileQuitting(t *testing.T) {
	d := newFakeDaemon(core.ForwardSession{Status: core.Active})
	h := newHarness(t, d)
	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	h.life.RequestQuit(false)
	h.life.RequestQuit(true)

	if len(h.dialog.quitAsks) != 0 {
		t.Errorf("終了処理中に確認した: %v", h.dialog.quitAsks)
	}
}
