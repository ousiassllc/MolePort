package handler

import (
	"net/http"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
)

// Fragments はダッシュボードの読み取り専用フラグメント（/fragments/{banner,header,hosts,forwards,status,toasts}）。
// すべて GET で HX-Request: true が必要（HXRequired ミドルウェアが検査する）。
type Fragments struct {
	life   service.LifecycleSource
	dash   service.Dashboard
	toasts *service.NotificationQueue
}

// NewFragments は Fragments を返す。
func NewFragments(life service.LifecycleSource, dash service.Dashboard, toasts *service.NotificationQueue) *Fragments {
	return &Fragments{life: life, dash: dash, toasts: toasts}
}

// Banner は #status-banner-region の内容を返す。ready のときは空（バナーを消す）。
func (f *Fragments) Banner(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, organisms.StatusBanner(f.life.State()))
}

// Header は #header の内容を返す。
func (f *Fragments) Header(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, organisms.Header(f.dash.Header()))
}

// Hosts は #hosts-panel の内容を返す。
func (f *Fragments) Hosts(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, organisms.HostPanel(f.dash.Hosts(), f.ready()))
}

// Forwards は #forwards-panel の内容を返す。
func (f *Fragments) Forwards(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, organisms.ForwardPanel(f.dash.Forwards(), f.ready()))
}

// Status は #status-bar の内容を返す。
func (f *Fragments) Status(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, organisms.StatusBar(f.dash.Status()))
}

// Toasts は NotificationQueue を取り出し（drain）、OOB の beforeend:#toast-stack として返す。
// 空なら 204。取り出し時にキューを消すため、このフラグメントのみ読み取りで状態が変わる。
func (f *Fragments) Toasts(w http.ResponseWriter, r *http.Request) {
	toasts := f.toasts.Drain()
	if len(toasts) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	render(w, r, http.StatusOK, organisms.Toasts(toasts))
}

func (f *Fragments) ready() bool { return f.life.State() == service.LifecycleReady }
