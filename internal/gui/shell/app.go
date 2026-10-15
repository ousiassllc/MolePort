//go:build gui

package shell

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	trayassets "github.com/ousiassllc/moleport/assets/tray"
	"github.com/ousiassllc/moleport/internal/core/guiconfig"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/bridge"
	"github.com/ousiassllc/moleport/internal/gui/service/credential"
	"github.com/ousiassllc/moleport/internal/gui/service/settings"
	"github.com/ousiassllc/moleport/internal/gui/service/trayops"
	"github.com/ousiassllc/moleport/internal/gui/tray"
	"github.com/ousiassllc/moleport/internal/gui/web"
	"github.com/ousiassllc/moleport/internal/gui/web/middleware"
)

const (
	windowTitle = "MolePort"
	// singleInstanceID は SingleInstanceLock の識別子。GUI の 2 プロセス目はこの ID で検知される（UC-38）。
	singleInstanceID = "io.github.ousiassllc.moleport-gui"
)

// Config は Run に渡す起動設定。
type Config struct {
	ConfigDir string
	Version   string
	Static    fs.FS           // go:embed の dist 含む FS
	Theme     guiconfig.Theme // 起動時に読んだ gui.theme（内蔵デーモンの起動前に描画するページで使う）
}

// Run は Wails を起動し、ウィンドウが閉じて終了するまでブロックする。
// Wails の型はこのパッケージの内側に閉じ込め、Web / Service 層へは service.Notifier と
// service.LifecycleSource だけを渡す。
func Run(cfg Config) error {
	token, err := middleware.NewToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	a := &app{cfg: cfg, queue: service.NewNotificationQueue(), activity: service.NewActivity()}
	a.notifier = NewVisibilityNotifier(a.emit)
	broker := credential.NewBroker(a.notifier, windowShower(a.Show))
	a.life = NewLifecycle(cfg.ConfigDir, cfg.Version, LifecycleDeps{
		Takeover:  daemon.Takeover,
		NewDaemon: func(dir, ver string) (DaemonHost, error) { return daemon.New(dir, ver) },
		Notifier:  a.notifier,
		Window:    a,
		Dialog:    a,
		Queue:     a.queue,
		OnReady:   a.onDaemonReady,
	})
	a.notice = NewTrayNotice(a.life, a)
	a.closer = NewCloseController(a, a, a.life, a.trayAvailable.Load, a.notice.Show)

	dash := service.NewDashboardService(a.life, a.activity)
	forwards := service.NewForwardService(a.life, broker, a.activity, a.queue, a.notifier)
	a.trayMenu = func() tray.Menu { return tray.BuildFrom(dash) }
	a.trayOps = trayops.New(dash, forwards, a.queue, a.notifier)

	handler := web.NewRouter(web.Deps{
		Life:      a.life,
		Dashboard: dash,
		Toasts:    a.queue,
		Forwards:  forwards,
		Hosts:     service.NewHostService(a.life, broker, a.activity, a.queue, a.notifier),
		Cred:      broker,
		Settings:  settingsHook{Settings: settings.New(a.life, a), onSaved: a.refreshTrayNow},
		Theme:     cfg.Theme,
		Token:     token,
		Static:    cfg.Static,
	})

	return wails.Run(&options.App{
		Title:       windowTitle,
		Width:       1000,
		Height:      700,
		MinWidth:    800,
		MinHeight:   560,
		StartHidden: false,
		AssetServer: &assetserver.Options{Handler: handler},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { a.Show() },
		},
		OnStartup:     a.onStartup,
		OnDomReady:    func(context.Context) { a.notifier.SetVisible(true) },
		OnBeforeClose: func(context.Context) bool { return a.closer.BeforeClose() },
		OnShutdown:    a.onShutdown,
	})
}

// windowShower は関数を credential.WindowController にする。
type windowShower func()

func (f windowShower) Show() { f() }

// app は Wails のコールバックと Shell の純粋ロジックを結ぶ。
type app struct {
	cfg      Config
	notifier *VisibilityNotifier
	queue    *service.NotificationQueue
	activity *service.Activity
	life     *Lifecycle
	closer   *CloseController
	notice   *TrayNotice
	trayMenu func() tray.Menu
	trayOps  *trayops.Actions

	ctxMu sync.RWMutex
	ctx   context.Context

	trayAvailable atomic.Bool
	notifyOK      atomic.Bool
	trayMu        sync.Mutex
	trayHandle    *tray.Tray
}

func (a *app) context() context.Context {
	a.ctxMu.RLock()
	defer a.ctxMu.RUnlock()
	return a.ctx
}

// onStartup は Wails の OnStartup。WebView の読み込みをブロックしないよう、
// 内蔵デーモンの起動とトレイの初期化は別ゴルーチンで行う。
func (a *app) onStartup(ctx context.Context) {
	a.ctxMu.Lock()
	a.ctx = ctx
	a.ctxMu.Unlock()

	a.initNotifications()
	go a.startTray()
	go a.life.Run(ctx)
	go a.watchSignals(ctx)
}

// watchSignals は OS シグナル（SIGINT / SIGTERM）を確認なしの終了要求に変換する。
func (a *app) watchSignals(ctx context.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ch)
	select {
	case <-ch:
		a.life.RequestQuit(false)
	case <-ctx.Done():
	}
}

// onDaemonReady は内蔵デーモンの起動完了時に EventBridge とトレイの追従を開始する。
func (a *app) onDaemonReady(ctx context.Context, b service.Backend) {
	bridge.New(b.SSH, b.Forward, a.notifier, a.queue, a.notifier).Start(ctx)
	// 非表示中は EventBridge が通知を止めるため、トレイは独自に購読して追従する（NFR-33）。
	go tray.Watch(ctx, b.SSH, b.Forward, tray.ReconcileInterval, a.refreshTray)
	a.refreshTrayNow()
}

func (a *app) startTray() {
	icon := trayassets.PNG
	if runtime.GOOS == "windows" {
		icon = trayassets.ICO
	}
	t, ok := tray.Start(tray.Config{
		TemplateIcon: trayassets.MacTemplate,
		Icon:         icon,
		Tooltip:      windowTitle,
		Build:        a.trayMenu,
		OnShow:       a.Show,
		OnToggle:     a.trayOps.ToggleForward,
		OnStopAll:    a.trayOps.StopAll,
		OnQuit:       func() { a.life.RequestQuit(true) },
	})
	a.trayMu.Lock()
	a.trayHandle = t
	a.trayMu.Unlock()
	a.trayAvailable.Store(ok)
}

// refreshTray はトレイメニューの更新を予約する（500ms デバウンス）。トレイ起動前は何もしない。
func (a *app) refreshTray() {
	if t := a.tray(); t != nil {
		t.Refresh()
	}
}

// refreshTrayNow はトレイメニューを即時に更新する（起動完了・言語変更）。トレイ起動前は何もしない。
func (a *app) refreshTrayNow() {
	if t := a.tray(); t != nil {
		t.RefreshNow()
	}
}

func (a *app) tray() *tray.Tray {
	a.trayMu.Lock()
	defer a.trayMu.Unlock()
	return a.trayHandle
}

func (a *app) stopTray() {
	if t := a.tray(); t != nil {
		t.Stop()
	}
}

func (a *app) onShutdown(context.Context) {
	a.life.Shutdown()
	a.stopTray()
}

func (a *app) emit(topics []string) {
	wailsruntime.EventsEmit(a.context(), ChangedEvent, map[string][]string{"topics": topics})
}
