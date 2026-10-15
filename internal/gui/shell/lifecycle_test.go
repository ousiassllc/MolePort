package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/gui/service"
)

func TestLifecycle_StartSuccess(t *testing.T) {
	h := newHarness(t, newFakeDaemon())
	if got := h.life.State(); got != service.LifecycleStarting {
		t.Fatalf("初期 State = %q, want starting", got)
	}

	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := h.life.State(); got != service.LifecycleReady {
		t.Errorf("State = %q, want ready", got)
	}
	if len(h.notifier.got) != 1 || h.notifier.got[0] != service.TopicAll {
		t.Errorf("ready 遷移時の通知 = %v, want [all]", h.notifier.got)
	}
}

func TestLifecycle_BackendAvailableOnlyWhenReady(t *testing.T) {
	d := newFakeDaemon()
	h := newHarness(t, d)
	if _, ok := h.life.Backend(); ok {
		t.Fatal("起動前に Backend が返ってはならない")
	}

	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	b, ok := h.life.Backend()
	if !ok {
		t.Fatal("ready 後に Backend が返らない")
	}
	if b.Forward != d.fwd {
		t.Error("Backend.Forward がデーモンの ForwardManager でない")
	}
	if b.Info.Version != "1.2.3" || b.Info.PID != os.Getpid() {
		t.Errorf("Backend.Info = %+v", b.Info)
	}
	if len(h.ready) != 1 || h.ready[0].Forward != d.fwd {
		t.Errorf("OnReady の呼び出し = %+v, want ready 遷移時に 1 回", h.ready)
	}
}

func TestLifecycle_BackendUnavailableAfterFailedStart(t *testing.T) {
	h := newHarness(t, newFakeDaemon())
	h.takeErr = errors.New("boom")

	_ = h.life.Start(context.Background())

	if _, ok := h.life.Backend(); ok {
		t.Error("起動失敗後に Backend が返ってはならない")
	}
	if len(h.ready) != 0 {
		t.Error("起動失敗時に OnReady を呼んではならない")
	}
}

func TestLifecycle_TakeoverPushesToast(t *testing.T) {
	tests := []struct {
		name    string
		prevPID int
		want    int
	}{
		{"引き継ぎあり", 1234, 1},
		{"引き継ぎなし", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, newFakeDaemon())
			h.takePID = tt.prevPID

			if err := h.life.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}

			toasts := h.queue.Drain()
			if len(toasts) != tt.want {
				t.Fatalf("toasts = %+v, want %d 件", toasts, tt.want)
			}
			if tt.want == 1 && toasts[0].Kind != service.ToastInfo {
				t.Errorf("Kind = %q, want info", toasts[0].Kind)
			}
		})
	}
}

func TestLifecycle_StartFailures(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(h *harness)
		wantErr error
		wantKey string
	}{
		{
			name:    "引き継ぎ: 応答なし",
			setup:   func(h *harness) { h.takeErr = daemon.ErrTakeoverUnresponsive },
			wantErr: daemon.ErrTakeoverUnresponsive,
			wantKey: "gui.dialog.takeover_unresponsive",
		},
		{
			name:    "引き継ぎ: タイムアウト",
			setup:   func(h *harness) { h.takeErr = daemon.ErrTakeoverTimeout },
			wantErr: daemon.ErrTakeoverTimeout,
			wantKey: "gui.dialog.takeover_timeout",
		},
		{
			name:    "デーモン生成失敗",
			setup:   func(h *harness) { h.newErr = errors.New("boom") },
			wantKey: "gui.dialog.startup_failed",
		},
		{
			name:    "デーモン起動失敗",
			setup:   func(h *harness) { h.daemon.startErr = errors.New("pid locked") },
			wantKey: "gui.dialog.startup_failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, newFakeDaemon())
			tt.setup(h)

			err := h.life.Start(context.Background())

			if err == nil {
				t.Fatal("Start = nil, want error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want wrapping %v", err, tt.wantErr)
			}
			if got := StartupErrorKey(err); got != tt.wantKey {
				t.Errorf("StartupErrorKey = %q, want %q", got, tt.wantKey)
			}
			if got := h.life.State(); got != service.LifecycleFailed {
				t.Errorf("State = %q, want failed", got)
			}
			if len(h.notifier.got) != 0 {
				t.Errorf("失敗時は通知しない: %v", h.notifier.got)
			}
			if h.daemon.stopCount() != 0 {
				t.Error("起動失敗時に Stop を呼ぶと state.yaml を空で上書きしうる")
			}
		})
	}
}

func TestLifecycle_DaemonDoneTriggersQuitWithoutConfirm(t *testing.T) {
	d := newFakeDaemon(core.ForwardSession{Status: core.Active})
	h := newHarness(t, d)
	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	d.closeDone() // daemon.shutdown (IPC) 相当

	h.waitQuit(t)
	if !h.life.Quitting() {
		t.Error("Quitting = false, want true")
	}
	if len(h.dialog.quitAsks) != 0 {
		t.Errorf("Done 起因の終了で確認した: %v", h.dialog.quitAsks)
	}
}

func TestLifecycle_RequestQuit(t *testing.T) {
	active := core.ForwardSession{Status: core.Active}
	stopped := core.ForwardSession{Status: core.Stopped}

	tests := []struct {
		name         string
		sessions     []core.ForwardSession
		confirm      bool // ConfirmQuit の応答
		requestConf  bool // RequestQuit の引数
		wantQuit     bool
		wantConfirms []int
	}{
		{name: "confirm=false は確認しない", sessions: []core.ForwardSession{active}, requestConf: false, wantQuit: true},
		{name: "confirm=true でもアクティブ転送なしなら確認しない", sessions: []core.ForwardSession{stopped}, requestConf: true, wantQuit: true},
		{name: "アクティブ転送あり・承認", sessions: []core.ForwardSession{active, stopped, active}, confirm: true, requestConf: true, wantQuit: true, wantConfirms: []int{2}},
		{name: "アクティブ転送あり・拒否", sessions: []core.ForwardSession{active}, confirm: false, requestConf: true, wantQuit: false, wantConfirms: []int{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newFakeDaemon(tt.sessions...)
			h := newHarness(t, d)
			h.dialog.confirm = tt.confirm
			if err := h.life.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}

			h.life.RequestQuit(tt.requestConf)

			if tt.wantQuit {
				h.waitQuit(t)
				if d.stopCount() < 1 {
					t.Error("Quit 前にデーモンが停止されていない")
				}
				if !h.life.Quitting() {
					t.Error("Quitting = false")
				}
			} else {
				h.noQuit(t)
				if h.life.Quitting() || d.stopCount() != 0 {
					t.Error("拒否されたのに終了処理が進んだ")
				}
			}
			if fmt.Sprint(h.dialog.quitAsks) != fmt.Sprint(tt.wantConfirms) {
				t.Errorf("confirms = %v, want %v", h.dialog.quitAsks, tt.wantConfirms)
			}
		})
	}
}

func TestLifecycle_RequestQuitIsIdempotent(t *testing.T) {
	h := newHarness(t, newFakeDaemon())
	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	h.life.RequestQuit(false)
	h.life.RequestQuit(false)
	h.waitQuit(t)

	h.noQuit(t) // 2 回目以降（Done 監視経由を含む）では Quit を再度呼ばない
}

func TestLifecycle_RequestQuitBeforeDaemonStarted(t *testing.T) {
	h := newHarness(t, newFakeDaemon())

	h.life.RequestQuit(true) // 起動失敗ダイアログ後の終了など。デーモンなしでも Quit する

	h.waitQuit(t)
	if h.daemon.stopCount() != 0 {
		t.Error("未起動のデーモンを Stop した")
	}
}

func TestLifecycle_ShutdownStopsDaemonWithoutQuit(t *testing.T) {
	d := newFakeDaemon()
	h := newHarness(t, d)
	if err := h.life.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	h.life.Shutdown()
	h.life.Shutdown()

	if d.stopCount() == 0 {
		t.Error("Shutdown がデーモンを停止していない")
	}
	if !h.life.Quitting() {
		t.Error("Quitting = false, want true")
	}
	h.noQuit(t) // OnShutdown 経路では Wails の Quit を再度呼ばない
}
