package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func do(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestChain_OrderOuterToInner(t *testing.T) {
	var order []string
	mk := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	}), mk("a"), mk("b"))

	do(h, http.MethodGet, "/", nil)

	if got := strings.Join(order, ","); got != "a,b,handler" {
		t.Errorf("order = %q, want %q", got, "a,b,handler")
	}
}

func TestNewToken(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if len(a) != 64 {
		t.Errorf("len(token) = %d, want 64 (32 バイトの 16 進表現)", len(a))
	}
	if a == b {
		t.Error("NewToken() が同じ値を返した（起動ごとにランダムであるべき）")
	}
}

func TestToken(t *testing.T) {
	const token = "secret-token"
	h := Token(token)(ok)

	tests := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"GET はトークン不要", http.MethodGet, nil, http.StatusOK},
		{"HEAD はトークン不要", http.MethodHead, nil, http.StatusOK},
		{"POST トークンなしは 403", http.MethodPost, nil, http.StatusForbidden},
		{"POST 不一致は 403", http.MethodPost, map[string]string{TokenHeader: "wrong"}, http.StatusForbidden},
		{"POST 前方一致だけでは 403", http.MethodPost, map[string]string{TokenHeader: "secret"}, http.StatusForbidden},
		{"POST 一致は通過", http.MethodPost, map[string]string{TokenHeader: token}, http.StatusOK},
		{"DELETE トークンなしは 403", http.MethodDelete, nil, http.StatusForbidden},
		{"DELETE 一致は通過", http.MethodDelete, map[string]string{TokenHeader: token}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := do(h, tt.method, "/actions/x", tt.headers).Code; got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestToken_EmptyConfiguredTokenRejectsAll(t *testing.T) {
	h := Token("")(ok)
	if got := do(h, http.MethodPost, "/actions/x", nil).Code; got != http.StatusForbidden {
		t.Errorf("status = %d, want 403（空トークン同士の一致を許してはならない）", got)
	}
}

func TestHXRequired(t *testing.T) {
	h := HXRequired(ok)

	tests := []struct {
		name    string
		target  string
		headers map[string]string
		want    int
	}{
		{"フラグメントを HX-Request なしで直接アクセス", "/fragments/hosts", nil, http.StatusBadRequest},
		{"HX-Request が true 以外", "/fragments/hosts", map[string]string{"HX-Request": "false"}, http.StatusBadRequest},
		{"フラグメントを HX-Request 付きで取得", "/fragments/hosts", map[string]string{"HX-Request": "true"}, http.StatusOK},
		{"フラグメント以外は不要", "/", nil, http.StatusOK},
		{"静的アセットは不要", "/static/dist/js/app.js", nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := do(h, http.MethodGet, tt.target, tt.headers).Code; got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(ok)

	t.Run("HTML は CSP・nosniff・no-store", func(t *testing.T) {
		rec := do(h, http.MethodGet, "/", nil)
		if got := rec.Header().Get("Content-Security-Policy"); got != ContentSecurityPolicy {
			t.Errorf("CSP = %q, want %q", got, ContentSecurityPolicy)
		}
		if !strings.Contains(ContentSecurityPolicy, "script-src 'self'") {
			t.Errorf("CSP に script-src 'self' が含まれない: %q", ContentSecurityPolicy)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
	})

	t.Run("静的アセットは no-cache", func(t *testing.T) {
		rec := do(h, http.MethodGet, "/static/dist/js/app.js", nil)
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", got)
		}
		if rec.Header().Get("Content-Security-Policy") == "" {
			t.Error("静的アセットにも CSP が付与されるべき")
		}
	})
}

func TestRecover(t *testing.T) {
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom: /secret/path")
	}))

	rec := do(h, http.MethodGet, "/", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") || strings.Contains(rec.Body.String(), "/secret/path") {
		t.Errorf("本文に内部情報が含まれている: %q", rec.Body.String())
	}
}

func TestRecover_ErrAbortHandlerIsRepanicked(t *testing.T) {
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if r := recover(); r != http.ErrAbortHandler {
			t.Errorf("recover() = %v, want http.ErrAbortHandler", r)
		}
	}()
	do(h, http.MethodGet, "/", nil)
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLogging_RecordsMethodAndPathOnly(t *testing.T) {
	buf := captureLogs(t)

	if got := do(Logging(ok), http.MethodGet, "/fragments/hosts?token=abc", nil).Code; got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}

	out := buf.String()
	if !strings.Contains(out, "/fragments/hosts") {
		t.Errorf("パスが記録されていない: %q", out)
	}
	if strings.Contains(out, "token=abc") {
		t.Errorf("クエリが記録されている: %q", out)
	}
}

func TestLogging_ExcludesCredentialRoutes(t *testing.T) {
	buf := captureLogs(t)

	for _, target := range []string{"/actions/credential/abc", "/actions/credential/abc/cancel"} {
		if got := do(Logging(ok), http.MethodPost, target, nil).Code; got != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", target, got)
		}
	}

	if buf.Len() != 0 {
		t.Errorf("クレデンシャルルートが記録されている: %q", buf.String())
	}
}
