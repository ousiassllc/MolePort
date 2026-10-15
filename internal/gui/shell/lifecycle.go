package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// DaemonHost は Lifecycle が内蔵デーモンに要求する操作。*daemon.Daemon が満たす。
type DaemonHost interface {
	Start(ctx context.Context) error
	Stop() error
	Done() <-chan struct{}
	Services() daemon.Services
}

// LifecycleDeps は Lifecycle の外部依存。Wails・実デーモンを差し替え可能にする。
type LifecycleDeps struct {
	// Takeover は稼働中の headless デーモンを引き継ぐ（daemon.Takeover）。
	Takeover func(configDir string, timeout time.Duration) (prevPID int, err error)
	// NewDaemon は内蔵デーモンを生成する（daemon.New のラッパー）。
	NewDaemon func(configDir, version string) (DaemonHost, error)
	// Notifier は更新トピックの通知先。ready 遷移時に all を送る。
	Notifier service.Notifier
	// Quit は Wails の終了フロー（runtime.Quit）を起動する。
	Quit func()
	// ConfirmQuit はアクティブな転送が active 件ある場合の終了確認。承認されたら true。
	ConfirmQuit func(active int) bool
	// Queue は非同期通知の積み先。引き継ぎ完了トーストに使う（nil なら積まない）。
	Queue *service.NotificationQueue
	// OnReady は内蔵デーモンの起動完了直後（ready 遷移前）に呼ばれる。EventBridge の開始に使う。
	// ctx は Start に渡された ctx（アプリ終了で cancel される）。
	OnReady func(ctx context.Context, b service.Backend)
}

// Lifecycle は GUI プロセスの起動・引き継ぎ・終了を統括する。
// 仕様は docs/architecture/gui.md「起動シーケンス」「終了シーケンス」。
type Lifecycle struct {
	configDir string
	version   string
	deps      LifecycleDeps

	quitting atomic.Bool

	mu    sync.Mutex
	state service.LifecycleState
	host  DaemonHost
	back  service.Backend
}

var (
	_ service.LifecycleSource = (*Lifecycle)(nil)
	_ service.BackendSource   = (*Lifecycle)(nil)
)

// NewLifecycle は starting 状態の Lifecycle を返す。
func NewLifecycle(configDir, version string, deps LifecycleDeps) *Lifecycle {
	return &Lifecycle{
		configDir: configDir,
		version:   version,
		deps:      deps,
		state:     service.LifecycleStarting,
	}
}

// State は内蔵デーモンの起動状態を返す（web.Deps.Life）。
func (l *Lifecycle) State() service.LifecycleState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state
}

// Backend は内蔵デーモン起動後の Core 参照を返す（service.BackendSource）。ready になるまでは ok=false。
func (l *Lifecycle) Backend() (service.Backend, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.back, l.state == service.LifecycleReady
}

func (l *Lifecycle) newBackend(host DaemonHost) service.Backend {
	svc := host.Services()
	b := service.Backend{
		SSH:     svc.SSH,
		Forward: svc.Forward,
		Config:  svc.Config,
		Info:    service.Info{Version: l.version, PID: os.Getpid()},
	}
	if svc.Version != nil { // nil の *VersionChecker を interface に入れない
		b.Version = svc.Version
	}
	return b
}

// Quitting は終了処理中かを返す。OnBeforeClose がウィンドウを閉じるかの判定に使う。
func (l *Lifecycle) Quitting() bool { return l.quitting.Load() }

// Start は引き継ぎ → 内蔵デーモン起動 → ready 遷移を行う。ブロックするため、
// WebView の読み込みを妨げないよう呼び出し側はゴルーチンで呼ぶ。
// ctx は Wails の OnStartup の ctx（アプリ終了で cancel され、デーモンも停止する）。
func (l *Lifecycle) Start(ctx context.Context) error {
	prevPID, err := l.deps.Takeover(l.configDir, daemon.TakeoverTimeout)
	if err != nil {
		return l.fail(fmt.Errorf("takeover: %w", err))
	}

	host, err := l.deps.NewDaemon(l.configDir, l.version)
	if err != nil {
		return l.fail(fmt.Errorf("create daemon: %w", err))
	}
	if err := host.Start(ctx); err != nil {
		return l.fail(fmt.Errorf("start daemon: %w", err))
	}

	back := l.newBackend(host)
	if l.deps.OnReady != nil {
		l.deps.OnReady(ctx, back)
	}
	if prevPID > 0 && l.deps.Queue != nil {
		l.deps.Queue.Push(service.ToastView{Kind: service.ToastInfo, Message: i18n.T("gui.toast.takeover")})
	}

	l.mu.Lock()
	l.host = host
	l.back = back
	l.state = service.LifecycleReady
	l.mu.Unlock()

	go l.watchDone(host)
	l.deps.Notifier.Notify(service.TopicAll)
	return nil
}

func (l *Lifecycle) fail(err error) error {
	l.mu.Lock()
	l.state = service.LifecycleFailed
	l.mu.Unlock()
	return err
}

// watchDone は daemon.shutdown (IPC) 等によるデーモンの終了を Wails の終了フローへ接続する。
// この経路は確認なし（CLI/TUI からの明示的な停止要求のため）。
func (l *Lifecycle) watchDone(host DaemonHost) {
	<-host.Done()
	l.RequestQuit(false)
}

// RequestQuit は終了を要求する。confirm が true でアクティブな転送がある場合は確認ダイアログを出し、
// 拒否されたら何もしない。承認（または confirm=false）後は quitting を立て、
// デーモンを停止してから Wails の終了フローを起動する。2 回目以降の呼び出しは無視する。
func (l *Lifecycle) RequestQuit(confirm bool) {
	if confirm && l.deps.ConfirmQuit != nil {
		if n := l.ActiveForwards(); n > 0 && !l.deps.ConfirmQuit(n) {
			return
		}
	}
	if !l.quitting.CompareAndSwap(false, true) {
		return
	}
	l.stopDaemon()
	l.deps.Quit()
}

// Shutdown は OnShutdown でデーモン停止を保証する。Quit を再度呼ばない。Stop は冪等。
func (l *Lifecycle) Shutdown() {
	l.quitting.Store(true)
	l.stopDaemon()
}

func (l *Lifecycle) stopDaemon() {
	l.mu.Lock()
	host := l.host
	l.mu.Unlock()
	if host == nil {
		return
	}
	_ = host.Stop() // daemon.Stop は失敗してもログのみで常に nil を返す。冪等。
}

// ActiveForwards はアクティブな転送の件数を返す。デーモン未起動なら 0。
func (l *Lifecycle) ActiveForwards() int {
	l.mu.Lock()
	host := l.host
	l.mu.Unlock()
	if host == nil {
		return 0
	}
	n := 0
	for _, s := range host.Services().Forward.GetAllSessions() {
		if s.Status == core.Active {
			n++
		}
	}
	return n
}

// StartupErrorKey は起動失敗の原因に対応する i18n キー（エラーダイアログの本文）を返す。
func StartupErrorKey(err error) string {
	switch {
	case errors.Is(err, daemon.ErrTakeoverUnresponsive):
		return "gui.dialog.takeover_unresponsive"
	case errors.Is(err, daemon.ErrTakeoverTimeout):
		return "gui.dialog.takeover_timeout"
	default:
		return "gui.dialog.startup_failed"
	}
}
