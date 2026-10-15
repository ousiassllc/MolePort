package web

import (
	"mime"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/ousiassllc/moleport/internal/gui/service"
)

// switchableLife は状態を後から切り替えられる LifecycleSource（起動完了の遷移を再現する）。
type switchableLife struct{ state atomic.Value }

func newSwitchableLife(s service.LifecycleState) *switchableLife {
	l := &switchableLife{}
	l.state.Store(s)
	return l
}

func (l *switchableLife) State() service.LifecycleState {
	return l.state.Load().(service.LifecycleState)
}

// 起動中のページが表示されたまま ready になっても、/fragments/banner の再取得でバナーが消える。
func TestRouter_BannerFragment_FollowsLifecycle(t *testing.T) {
	life := newSwitchableLife(service.LifecycleStarting)
	h := NewRouter(Deps{
		Life:      life,
		Dashboard: &fakeDashboard{},
		Toasts:    service.NewNotificationQueue(),
		Settings:  &fakeSettings{},
		Token:     testToken,
		Static:    fstest.MapFS{"dist/.gitkeep": {}},
	})
	hx := map[string]string{"HX-Request": "true"}

	rec := serve(h, http.MethodGet, "/fragments/banner", hx)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `data-state="starting"`) {
		t.Fatalf("starting: status=%d body=%q", rec.Code, rec.Body.String())
	}

	life.state.Store(service.LifecycleFailed)
	if body := serve(h, http.MethodGet, "/fragments/banner", hx).Body.String(); !strings.Contains(body, `data-state="failed"`) {
		t.Errorf("failed: body=%q", body)
	}

	life.state.Store(service.LifecycleReady)
	rec = serve(h, http.MethodGet, "/fragments/banner", hx)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready: status=%d", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "status-banner") {
		t.Errorf("ready のときバナーを返してはならない: %q", body)
	}

	if got := serve(h, http.MethodGet, "/fragments/banner", nil).Code; got != http.StatusBadRequest {
		t.Errorf("HX-Request なし: status = %d, want 400", got)
	}
}

// Windows ではレジストリが .js / .css を text/plain 等に登録していることがある。
// nosniff + <script type="module"> が動くよう、Content-Type は OS の登録に依存しない。
func TestRouter_Static_ContentTypeIgnoresOSMimeRegistry(t *testing.T) {
	for ext, orig := range map[string]string{".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8"} {
		if err := mime.AddExtensionType(ext, "text/plain; charset=utf-8"); err != nil {
			t.Fatalf("AddExtensionType: %v", err)
		}
		t.Cleanup(func() { _ = mime.AddExtensionType(ext, orig) })
	}
	h := NewRouter(Deps{
		Life:      fakeLife{state: service.LifecycleReady},
		Dashboard: &fakeDashboard{},
		Toasts:    service.NewNotificationQueue(),
		Settings:  &fakeSettings{},
		Token:     testToken,
		Static: fstest.MapFS{
			"dist/js/app.js":      {Data: []byte("1")},
			"dist/js/app.js.map":  {Data: []byte("{}")},
			"dist/css/app.css":    {Data: []byte("body{}")},
			"dist/js/htmx.min.js": {Data: []byte("2")},
		},
	})

	for target, want := range map[string]string{
		"/static/dist/js/app.js":      "text/javascript; charset=utf-8",
		"/static/dist/js/htmx.min.js": "text/javascript; charset=utf-8",
		"/static/dist/css/app.css":    "text/css; charset=utf-8",
		"/static/dist/js/app.js.map":  "application/json",
	} {
		if got := serve(h, http.MethodGet, target, nil).Header().Get("Content-Type"); got != want {
			t.Errorf("GET %s: Content-Type = %q, want %q", target, got, want)
		}
	}
}
