package shell

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/gui/service"
)

type fakeForwardManager struct {
	core.ForwardManager
	sessions []core.ForwardSession
}

func (f *fakeForwardManager) GetAllSessions() []core.ForwardSession { return f.sessions }

type fakeDaemon struct {
	startErr error
	fwd      *fakeForwardManager

	mu        sync.Mutex
	stops     int
	done      chan struct{}
	closeOnce sync.Once
}

func newFakeDaemon(sessions ...core.ForwardSession) *fakeDaemon {
	return &fakeDaemon{fwd: &fakeForwardManager{sessions: sessions}, done: make(chan struct{})}
}

func (f *fakeDaemon) Start(context.Context) error { return f.startErr }
func (f *fakeDaemon) Stop() error {
	f.mu.Lock()
	f.stops++
	f.mu.Unlock()
	f.closeDone()
	return nil
}
func (f *fakeDaemon) Done() <-chan struct{}     { return f.done }
func (f *fakeDaemon) closeDone()                { f.closeOnce.Do(func() { close(f.done) }) }
func (f *fakeDaemon) Services() daemon.Services { return daemon.Services{Forward: f.fwd} }
func (f *fakeDaemon) stopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stops
}

type recordingNotifier struct {
	mu  sync.Mutex
	got []service.Topic
}

func (r *recordingNotifier) Notify(topics ...service.Topic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, topics...)
}

type harness struct {
	life     *Lifecycle
	daemon   *fakeDaemon
	notifier *recordingNotifier
	quits    chan struct{}
	confirms []int
	confirm  bool
	takeErr  error
	takePID  int
	newErr   error
	queue    *service.NotificationQueue
	ready    []service.Backend
}

func newHarness(t *testing.T, d *fakeDaemon) *harness {
	t.Helper()
	h := &harness{daemon: d, notifier: &recordingNotifier{}, quits: make(chan struct{}, 8), queue: service.NewNotificationQueue()}
	h.life = NewLifecycle("/cfg", "1.2.3", LifecycleDeps{
		Takeover: func(configDir string, _ time.Duration) (int, error) {
			if configDir != "/cfg" {
				t.Errorf("Takeover configDir = %q", configDir)
			}
			return h.takePID, h.takeErr
		},
		NewDaemon: func(configDir, version string) (DaemonHost, error) {
			if configDir != "/cfg" || version != "1.2.3" {
				t.Errorf("NewDaemon(%q, %q)", configDir, version)
			}
			if h.newErr != nil {
				return nil, h.newErr
			}
			return h.daemon, nil
		},
		Notifier: h.notifier,
		Queue:    h.queue,
		OnReady: func(_ context.Context, b service.Backend) {
			h.ready = append(h.ready, b)
		},
		Quit: func() { h.quits <- struct{}{} },
		ConfirmQuit: func(active int) bool {
			h.confirms = append(h.confirms, active)
			return h.confirm
		},
	})
	return h
}

func (h *harness) waitQuit(t *testing.T) {
	t.Helper()
	select {
	case <-h.quits:
	case <-time.After(2 * time.Second):
		t.Fatal("Quit が呼ばれなかった")
	}
}

func (h *harness) noQuit(t *testing.T) {
	t.Helper()
	select {
	case <-h.quits:
		t.Fatal("Quit が呼ばれた")
	case <-time.After(50 * time.Millisecond):
	}
}
