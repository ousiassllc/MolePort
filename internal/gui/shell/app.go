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
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/tray"
	"github.com/ousiassllc/moleport/internal/gui/web"
	"github.com/ousiassllc/moleport/internal/gui/web/middleware"
	"github.com/ousiassllc/moleport/internal/i18n"
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
	Static    fs.FS // go:embed の dist 含む FS
}

// Run は Wails を起動し、ウィンドウが閉じて終了するまでブロックする。
// Wails の型はこのパッケージの内側に閉じ込め、Web / Service 層へは service.Notifier と
// service.LifecycleSource だけを渡す。
func Run(cfg Config) error {
	token, err := middleware.NewToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	a := &app{cfg: cfg, queue: service.NewNotificationQueue()}
	a.notifier = NewVisibilityNotifier(a.emit)
	a.life = NewLifecycle(cfg.ConfigDir, cfg.Version, LifecycleDeps{
		Takeover:    daemon.Takeover,
		NewDaemon:   func(dir, ver string) (DaemonHost, error) { return daemon.New(dir, ver) },
		Notifier:    a.notifier,
		Quit:        a.quit,
		ConfirmQuit: a.confirmActiveQuit,
		Queue:       a.queue,
		OnReady: func(ctx context.Context, b service.Backend) {
			service.NewEventBridge(b.SSH, b.Forward, a.notifier, a.queue, a.notifier).Start(ctx)
		},
	})
	a.closer = NewCloseController(a, a.life.Quitting, a.trayAvailable.Load)

	handler := web.NewRouter(web.Deps{
		Life:      a.life,
		Dashboard: service.NewDashboardService(a.life),
		Toasts:    a.queue,
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
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { a.showWindow() },
		},
		OnStartup:     a.onStartup,
		OnDomReady:    func(context.Context) { a.notifier.SetVisible(true) },
		OnBeforeClose: func(context.Context) bool { return a.closer.BeforeClose() },
		OnShutdown:    a.onShutdown,
	})
}

// app は Wails のコールバックと Shell の純粋ロジックを結ぶ。
type app struct {
	cfg      Config
	notifier *VisibilityNotifier
	queue    *service.NotificationQueue
	life     *Lifecycle
	closer   *CloseController

	ctxMu sync.RWMutex
	ctx   context.Context

	trayAvailable atomic.Bool
	trayMu        sync.Mutex
	trayHandle    *tray.Tray
}

var _ CloseHost = (*app)(nil)

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

	go a.startTray()
	go a.startDaemon(ctx)
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

func (a *app) startDaemon(ctx context.Context) {
	if err := a.life.Start(ctx); err != nil {
		// 起動失敗: エラーダイアログを出してプロセスを終了する。
		a.showError(StartupErrorKey(err), err)
		a.life.RequestQuit(false)
	}
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
		ShowLabel:    i18n.T("gui.tray.show"),
		QuitLabel:    i18n.T("gui.tray.quit"),
		OnShow:       a.showWindow,
		OnQuit:       func() { a.life.RequestQuit(true) },
	})
	a.trayMu.Lock()
	a.trayHandle = t
	a.trayMu.Unlock()
	a.trayAvailable.Store(ok)
}

func (a *app) stopTray() {
	a.trayMu.Lock()
	t := a.trayHandle
	a.trayMu.Unlock()
	if t != nil {
		t.Stop()
	}
}

func (a *app) onShutdown(context.Context) {
	a.life.Shutdown()
	a.stopTray()
}

// quit は Lifecycle の Quit 依存。トレイを閉じてから Wails の終了フローを起動する。
func (a *app) quit() {
	a.stopTray()
	wailsruntime.Quit(a.context())
}

func (a *app) emit(topics []string) {
	wailsruntime.EventsEmit(a.context(), ChangedEvent, map[string][]string{"topics": topics})
}

func (a *app) showWindow() {
	ctx := a.context()
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
	a.notifier.SetVisible(true)
}

// HideWindow は CloseHost の実装。
func (a *app) HideWindow() {
	wailsruntime.WindowHide(a.context())
	a.notifier.SetVisible(false)
}

// ConfirmQuit は CloseHost の実装。トレイがない環境で閉じる操作を終了確認に倒す。
func (a *app) ConfirmQuit() bool {
	key := "gui.dialog.quit_confirm"
	data := map[string]any{}
	if n := a.life.ActiveForwards(); n > 0 {
		key = "gui.dialog.quit_confirm_active"
		data["Count"] = n
	}
	return a.askYesNo(i18n.T("gui.dialog.quit_confirm_title"), i18n.T(key, data))
}

// RequestQuit は CloseHost の実装。ダイアログで確認済みのため再確認しない。
func (a *app) RequestQuit() { a.life.RequestQuit(false) }

// confirmActiveQuit は Lifecycle の ConfirmQuit 依存（トレイ「終了」）。
func (a *app) confirmActiveQuit(active int) bool {
	return a.askYesNo(
		i18n.T("gui.dialog.quit_confirm_title"),
		i18n.T("gui.dialog.quit_active_confirm", map[string]any{"Count": active}),
	)
}

func (a *app) askYesNo(title, message string) bool {
	yes, no := i18n.T("gui.dialog.yes"), i18n.T("gui.dialog.no")
	result, err := wailsruntime.MessageDialog(a.context(), wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         title,
		Message:       message,
		Buttons:       []string{yes, no},
		DefaultButton: no,
		CancelButton:  no,
	})
	return err == nil && IsAffirmative(result, yes)
}

func (a *app) showError(key string, cause error) {
	_, _ = wailsruntime.MessageDialog(a.context(), wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.ErrorDialog,
		Title:   i18n.T("gui.dialog.startup_failed_title"),
		Message: i18n.T(key, map[string]any{"Error": cause.Error()}),
	})
}
