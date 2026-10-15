package trayops

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/ousiassllc/moleport/internal/gui/service"
)

type fakeTrayDashboard struct {
	service.Dashboard
	forwards []service.ForwardView
}

func (f fakeTrayDashboard) Forwards() []service.ForwardView { return f.forwards }

type fakeTrayForwards struct {
	service.Forwards
	started, stopped []string
	startErr         error
	stopErr          map[string]error
}

func (f *fakeTrayForwards) Start(name string) error {
	f.started = append(f.started, name)
	return f.startErr
}

func (f *fakeTrayForwards) Stop(name string) error {
	f.stopped = append(f.stopped, name)
	return f.stopErr[name]
}

type topicRecorder struct {
	mu  sync.Mutex
	got []service.Topic
}

func (r *topicRecorder) Notify(topics ...service.Topic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, topics...)
}

func newActions(views ...service.ForwardView) (*Actions, *fakeTrayForwards, *service.NotificationQueue, *topicRecorder) {
	fwd := &fakeTrayForwards{stopErr: map[string]error{}}
	q := service.NewNotificationQueue()
	n := &topicRecorder{}
	return New(fakeTrayDashboard{forwards: views}, fwd, q, n), fwd, q, n
}

func startable(name string) service.ForwardView {
	return service.ForwardView{Name: name, Actions: service.ForwardActions{CanStart: true}}
}

func stoppable(name string) service.ForwardView {
	return service.ForwardView{Name: name, Actions: service.ForwardActions{CanStop: true}}
}

func TestActions_ToggleForward(t *testing.T) {
	tests := []struct {
		name        string
		view        service.ForwardView
		wantStarted []string
		wantStopped []string
	}{
		{"停止中は開始", startable("web"), []string{"web"}, nil},
		{"稼働中は停止", stoppable("web"), nil, []string{"web"}},
		{"どちらも不可なら何もしない", service.ForwardView{Name: "web"}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, fwd, q, _ := newActions(tt.view)

			a.ToggleForward("web")

			if !slices.Equal(fwd.started, tt.wantStarted) || !slices.Equal(fwd.stopped, tt.wantStopped) {
				t.Errorf("started/stopped = %v/%v, want %v/%v", fwd.started, fwd.stopped, tt.wantStarted, tt.wantStopped)
			}
			if got := q.Drain(); len(got) != 0 {
				t.Errorf("成功時にトーストが積まれた: %+v", got)
			}
		})
	}
}

func TestActions_ToggleForwardUnknownRule(t *testing.T) {
	a, fwd, _, n := newActions(startable("web"))

	a.ToggleForward("missing")

	if len(fwd.started)+len(fwd.stopped) != 0 || len(n.got) != 0 {
		t.Errorf("存在しないルールで操作・通知した: %v %v %v", fwd.started, fwd.stopped, n.got)
	}
}

func TestActions_ToggleForwardReportsFailure(t *testing.T) {
	a, fwd, q, n := newActions(startable("web"))
	fwd.startErr = errors.New("boom")

	a.ToggleForward("web")

	toasts := q.Drain()
	if len(toasts) != 1 || toasts[0].Kind != service.ToastError {
		t.Fatalf("toasts = %+v, want error 1 件", toasts)
	}
	if !slices.Contains(n.got, service.TopicToast) {
		t.Errorf("TopicToast を通知していない: %v", n.got)
	}
}

func TestActions_ToggleForwardIgnoresStateConflict(t *testing.T) {
	a, fwd, q, _ := newActions(startable("web"))
	fwd.startErr = service.ErrStateConflict

	a.ToggleForward("web")

	if got := q.Drain(); len(got) != 0 {
		t.Errorf("状態競合でトーストが積まれた: %+v", got)
	}
}

func TestActions_StopAll(t *testing.T) {
	a, fwd, q, n := newActions(stoppable("a"), startable("b"), stoppable("c"), stoppable("d"))
	fwd.stopErr["a"] = errors.New("boom")

	a.StopAll()

	if want := []string{"a", "c", "d"}; !slices.Equal(fwd.stopped, want) {
		t.Errorf("stopped = %v, want %v（1 件の失敗で残りを止めない）", fwd.stopped, want)
	}
	if len(fwd.started) != 0 {
		t.Errorf("StopAll が開始した: %v", fwd.started)
	}
	if got := q.Drain(); len(got) != 1 {
		t.Errorf("toasts = %+v, want 失敗した 1 件", got)
	}
	if !slices.Contains(n.got, service.TopicForwards) {
		t.Errorf("TopicForwards を通知していない: %v", n.got)
	}
}
