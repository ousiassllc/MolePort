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

// fakeConfigManager は GetConfig / UpdateConfig だけを実装する core.ConfigManager のフェイク。
type fakeConfigManager struct {
	core.ConfigManager
	mu      sync.Mutex
	cfg     core.Config
	updates int
}

func (f *fakeConfigManager) GetConfig() *core.Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.cfg
	return &c
}

func (f *fakeConfigManager) UpdateConfig(fn func(*core.Config)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(&f.cfg)
	f.updates++
	return nil
}

type fakeDaemon struct {
	startErr error
	fwd      *fakeForwardManager
	cfg      core.ConfigManager

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
func (f *fakeDaemon) Done() <-chan struct{} { return f.done }
func (f *fakeDaemon) closeDone()            { f.closeOnce.Do(func() { close(f.done) }) }
func (f *fakeDaemon) Services() daemon.Services {
	return daemon.Services{Forward: f.fwd, Config: f.cfg}
}
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

// fakeWindow は Window のフェイク。Quit は quits に積む。
type fakeWindow struct {
	quits chan struct{}
	shows int
	hides int
}

func (w *fakeWindow) Show() { w.shows++ }
func (w *fakeWindow) Hide() { w.hides++ }
func (w *fakeWindow) Quit() {
	if w.quits != nil {
		w.quits <- struct{}{}
	}
}

// fakeDialog は Dialog のフェイク。呼ばれた確認と表示したエラーを記録する。
type fakeDialog struct {
	mu        sync.Mutex
	confirm   bool
	quitAsks  []int
	closeAsks []int
	errKeys   []string
	onQuitAsk func() // ConfirmQuit の応答前に呼ばれる（確認中の挙動を検証する）
}

func (d *fakeDialog) ConfirmQuit(active int) bool {
	d.mu.Lock()
	d.quitAsks = append(d.quitAsks, active)
	hook := d.onQuitAsk
	d.mu.Unlock()
	if hook != nil {
		hook()
	}
	return d.confirm
}

func (d *fakeDialog) ConfirmClose(active int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closeAsks = append(d.closeAsks, active)
	return d.confirm
}

func (d *fakeDialog) ShowError(key string, _ error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.errKeys = append(d.errKeys, key)
}

type harness struct {
	life     *Lifecycle
	daemon   *fakeDaemon
	notifier *recordingNotifier
	window   *fakeWindow
	dialog   *fakeDialog
	quits    chan struct{}
	takeErr  error
	takePID  int
	newErr   error
	queue    *service.NotificationQueue
	ready    []service.Backend
}

func newHarness(t *testing.T, d *fakeDaemon) *harness {
	t.Helper()
	quits := make(chan struct{}, 8)
	h := &harness{
		daemon: d, notifier: &recordingNotifier{}, quits: quits, queue: service.NewNotificationQueue(),
		window: &fakeWindow{quits: quits}, dialog: &fakeDialog{},
	}
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
		Window: h.window,
		Dialog: h.dialog,
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
