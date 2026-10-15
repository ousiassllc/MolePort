package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/handler"
	"github.com/ousiassllc/moleport/internal/gui/web/middleware"
)

type fakeLife struct{ state service.LifecycleState }

func (f fakeLife) State() service.LifecycleState { return f.state }

const testToken = "test-token-0123"

func newTestRouter(state service.LifecycleState) http.Handler {
	return NewRouter(Deps{
		Life:      fakeLife{state: state},
		Dashboard: &fakeDashboard{},
		Toasts:    service.NewNotificationQueue(),
		Settings:  &fakeSettings{notReady: state != service.LifecycleReady},
		Token:     testToken,
		Static: fstest.MapFS{
			"dist/js/app.js":   {Data: []byte("console.log('app')")},
			"dist/css/app.css": {Data: []byte("body{}")},
			"dist/.gitkeep":    {Data: nil},
		},
	})
}

func serve(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRouter_Index_BaseLayout(t *testing.T) {
	rec := serve(newTestRouter(service.LifecycleReady), http.MethodGet, "/", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<!doctype html>",
		"<head>",
		`<meta name="mp-token" content="` + testToken + `">`,
		`href="/static/dist/css/app.css"`,
		`src="/static/dist/js/htmx.min.js"`,
		`<script type="module" src="/static/dist/js/app.js">`,
		`id="modal-root"`,
		`id="toast-stack"`,
	} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Errorf("本文に %q が含まれない:\n%s", want, body)
		}
	}
}

func TestRouter_Index_LifecycleBanner(t *testing.T) {
	tests := []struct {
		state      service.LifecycleState
		wantBanner string
	}{
		{service.LifecycleStarting, `data-state="starting"`},
		{service.LifecycleFailed, `data-state="failed"`},
		{service.LifecycleReady, ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			body := serve(newTestRouter(tt.state), http.MethodGet, "/", nil).Body.String()

			if !strings.Contains(body, `data-lifecycle="`+string(tt.state)+`"`) {
				t.Errorf("data-lifecycle=%q が含まれない", tt.state)
			}
			hasBanner := strings.Contains(body, `id="status-banner"`)
			if tt.wantBanner == "" {
				if hasBanner {
					t.Error("ready のときバナーを描画してはならない")
				}
				return
			}
			if !hasBanner || !strings.Contains(body, tt.wantBanner) {
				t.Errorf("バナー %q が含まれない:\n%s", tt.wantBanner, body)
			}
		})
	}
}

func TestRouter_Index_OnlyRoot(t *testing.T) {
	h := newTestRouter(service.LifecycleReady)
	for _, target := range []string{"/unknown", "/index.html", "/actions"} {
		if got := serve(h, http.MethodGet, target, nil).Code; got != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", target, got)
		}
	}
}

func TestRouter_Headers_AllRoutes(t *testing.T) {
	h := newTestRouter(service.LifecycleReady)
	for _, target := range []string{"/", "/static/dist/js/app.js", "/unknown"} {
		rec := serve(h, http.MethodGet, target, nil)
		if got := rec.Header().Get("Content-Security-Policy"); got != middleware.ContentSecurityPolicy {
			t.Errorf("GET %s: CSP = %q", target, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("GET %s: X-Content-Type-Options = %q", target, got)
		}
	}
	if got := serve(h, http.MethodGet, "/", nil).Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("GET /: Cache-Control = %q, want no-store", got)
	}
}

func TestRouter_Static(t *testing.T) {
	h := newTestRouter(service.LifecycleReady)

	t.Run("fstest.MapFS から配信し no-cache", func(t *testing.T) {
		rec := serve(h, http.MethodGet, "/static/dist/js/app.js", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Body.String(); got != "console.log('app')" {
			t.Errorf("body = %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", got)
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "javascript") {
			t.Errorf("Content-Type = %q, want javascript", got)
		}
	})

	t.Run("CSS の Content-Type", func(t *testing.T) {
		rec := serve(h, http.MethodGet, "/static/dist/css/app.css", nil)
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
			t.Errorf("Content-Type = %q, want text/css", got)
		}
	})

	for _, tt := range []struct{ name, target string }{
		{"存在しないファイル", "/static/dist/js/none.js"},
		{"ディレクトリ", "/static/dist/js"},
		{"ディレクトリ（末尾スラッシュ）", "/static/dist/"},
		{"static 直下", "/static/"},
	} {
		t.Run(tt.name+"は 404（リダイレクトしない）", func(t *testing.T) {
			rec := serve(h, http.MethodGet, tt.target, nil)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
		})
	}
}

func TestRouter_StateChangingRequestsNeedToken(t *testing.T) {
	h := newTestRouter(service.LifecycleReady)

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			if got := serve(h, method, "/actions/unknown", nil).Code; got != http.StatusForbidden {
				t.Errorf("トークンなし: status = %d, want 403", got)
			}
			if got := serve(h, method, "/actions/unknown", map[string]string{middleware.TokenHeader: "wrong"}).Code; got != http.StatusForbidden {
				t.Errorf("不一致トークン: status = %d, want 403", got)
			}
			// トークンが一致すればミドルウェアを通過し、未定義ルートとして 404 になる。
			if got := serve(h, method, "/actions/unknown", map[string]string{middleware.TokenHeader: testToken}).Code; got != http.StatusNotFound {
				t.Errorf("一致トークン: status = %d, want 404（未定義ルート）", got)
			}
		})
	}
}

func TestRouter_FragmentsNeedHXRequest(t *testing.T) {
	h := newTestRouter(service.LifecycleReady)

	for _, target := range []string{"/fragments/hosts", "/fragments/unknown"} {
		if got := serve(h, http.MethodGet, target, nil).Code; got != http.StatusBadRequest {
			t.Errorf("GET %s HX-Request なし: status = %d, want 400", target, got)
		}
	}
	if got := serve(h, http.MethodGet, "/fragments/hosts", map[string]string{"HX-Request": "true"}).Code; got != http.StatusOK {
		t.Errorf("HX-Request あり（定義済みルート）: status = %d, want 200", got)
	}
	// HX-Request があればミドルウェアを通過し、未定義ルートとして 404 になる。
	if got := serve(h, http.MethodGet, "/fragments/unknown", map[string]string{"HX-Request": "true"}).Code; got != http.StatusNotFound {
		t.Errorf("HX-Request あり: status = %d, want 404（未定義ルート）", got)
	}
}

func TestRouter_StaticRejectsStateChangingMethods(t *testing.T) {
	h := newTestRouter(service.LifecycleReady)
	rec := serve(h, http.MethodPost, "/static/dist/js/app.js", map[string]string{middleware.TokenHeader: testToken})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestStatic_RejectsInvalidPath(t *testing.T) {
	s := handler.NewStatic(fstest.MapFS{"secret.txt": {Data: []byte("secret")}})
	for _, p := range []string{"../secret.txt", "/secret.txt", "a/../secret.txt", ""} {
		req := httptest.NewRequest(http.MethodGet, "/static/x", nil)
		req.SetPathValue("path", p)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("path %q: status = %d, want 404", p, rec.Code)
		}
	}
}
