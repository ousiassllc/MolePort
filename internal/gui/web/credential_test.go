package web

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/credential"
	"github.com/ousiassllc/moleport/internal/gui/web/handler"
	"github.com/ousiassllc/moleport/internal/gui/web/middleware"
)

// fakePrompter は credential.Prompter のテスト用実装。Resolve で受け取った値を記録してからゼロ化する。
type fakePrompter struct {
	pending  []service.CredentialView
	err      error
	calls    []string
	value    string
	answers  []string
	resolved credential.Input // ゼロ化の確認用（Resolve に渡されたバッファ）
}

func (f *fakePrompter) Pending() (service.CredentialView, bool) {
	if len(f.pending) == 0 {
		return service.CredentialView{}, false
	}
	return f.pending[0], true
}

func (f *fakePrompter) Resolve(id string, in credential.Input) error {
	defer in.Zero()
	f.calls = append(f.calls, "resolve:"+id)
	f.value = string(in.Value)
	for _, a := range in.Answers {
		f.answers = append(f.answers, string(a))
	}
	f.resolved = in
	if f.err != nil {
		return f.err
	}
	f.pending = f.pending[1:]
	return nil
}

func (f *fakePrompter) Cancel(id string) error {
	f.calls = append(f.calls, "cancel:"+id)
	if f.err != nil {
		return f.err
	}
	f.pending = f.pending[1:]
	return nil
}

var (
	passwordReq = service.CredentialView{RequestID: "gcr-1", Host: "prod", Type: "password"}
	kbdReq      = service.CredentialView{RequestID: "gcr-2", Host: "db", Type: "keyboard-interactive",
		Prompts: []service.CredentialPromptView{{Label: "Code: ", Echo: true}, {Label: "PIN: ", Echo: false}}}
)

func TestCredentialFragment(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleReady)

	rec := serve(e.h, http.MethodGet, "/fragments/credential", hxHeader)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "" {
		t.Errorf("待機なし: status = %d body=%q, want 200 で空", rec.Code, rec.Body.String())
	}

	e.cred.pending = []service.CredentialView{passwordReq}
	body := serve(e.h, http.MethodGet, "/fragments/credential", hxHeader).Body.String()
	for _, s := range []string{`data-request-id="gcr-1"`, `type="password"`, `name="value"`, `hx-post="/actions/credential/gcr-1"`,
		`hx-post="/actions/credential/gcr-1/cancel"`, "Authenticate to prod"} {
		if !strings.Contains(body, s) {
			t.Errorf("password: 本文に %q が無い:\n%s", s, body)
		}
	}

	e.cred.pending = []service.CredentialView{kbdReq}
	body = serve(e.h, http.MethodGet, "/fragments/credential", hxHeader).Body.String()
	if strings.Count(body, `name="answers[]"`) != 2 || !strings.Contains(body, `type="text"`) || !strings.Contains(body, `type="password"`) {
		t.Errorf("keyboard-interactive: プロンプト数・Echo の出し分けが不正:\n%s", body)
	}

	// 同じ要求のダイアログを表示中なら差し替えない（入力途中の値を保つ）。
	rec = serve(e.h, http.MethodGet, "/fragments/credential", map[string]string{"HX-Request": "true", handler.CredentialShownHeader: "gcr-2"})
	if rec.Code != http.StatusNoContent {
		t.Errorf("表示中と同じ要求: status = %d, want 204", rec.Code)
	}
}

// captureLogs は slog の既定ロガーを Debug レベルでバッファに向ける。
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &buf
}

func TestCredentialResolve_ValueOnlyInBodyAndNotLogged(t *testing.T) {
	setEnglish(t)
	logs := captureLogs(t)
	e := newActionEnv(service.LifecycleReady)
	e.cred.pending = []service.CredentialView{passwordReq, kbdReq}

	secret := "p@ss w%rd&=1" //nolint:gosec // テスト用のダミー値（エンコードが必要な文字を含む）
	rec := e.post(http.MethodPost, "/actions/credential/gcr-1", url.Values{"value": {secret}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if e.cred.value != secret {
		t.Errorf("Resolve の値 = %q, want %q", e.cred.value, secret)
	}
	if string(e.cred.resolved.Value) != strings.Repeat("\x00", len(secret)) {
		t.Errorf("Resolve 後のバッファがゼロ化されていない: %q", e.cred.resolved.Value)
	}
	// 次の待機要求のダイアログを返し、モーダルは閉じさせない。
	if !strings.Contains(rec.Body.String(), `data-request-id="gcr-2"`) || strings.Contains(rec.Header().Get("HX-Trigger"), "mp:close-modal") {
		t.Errorf("応答: HX-Trigger=%q body=%s", rec.Header().Get("HX-Trigger"), rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) || strings.Contains(logs.String(), "p@ss") || strings.Contains(logs.String(), "/actions/credential") {
		t.Errorf("クレデンシャル（またはルート）がログ・応答に出た: logs=%q", logs.String())
	}

	// keyboard-interactive は answers[] を順に渡す。
	rec = e.post(http.MethodPost, "/actions/credential/gcr-2", url.Values{"answers[]": {"123", "4 5"}})
	if rec.Code != http.StatusOK || strings.Join(e.cred.answers, ",") != "123,4 5" {
		t.Errorf("answers = %v status = %d", e.cred.answers, rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "" {
		t.Errorf("待機が無くなったら空を返す: %q", rec.Body.String())
	}
}

func TestCredentialResolve_ExpiredIs409(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleReady)
	e.cred.err = credential.ErrUnknownRequest

	for _, target := range []string{"/actions/credential/gcr-9", "/actions/credential/gcr-9/cancel"} {
		rec := e.post(http.MethodPost, target, url.Values{"value": {"x"}})
		if rec.Code != http.StatusConflict {
			t.Errorf("%s: status = %d, want 409", target, rec.Code)
		}
		if !strings.Contains(rec.Header().Get("HX-Trigger"), "mp:credential") || !strings.Contains(rec.Body.String(), "The input has expired.") {
			t.Errorf("%s: HX-Trigger=%q body=%s", target, rec.Header().Get("HX-Trigger"), rec.Body.String())
		}
	}
}

func TestCredentialCancel(t *testing.T) {
	e := newActionEnv(service.LifecycleReady)
	e.cred.pending = []service.CredentialView{passwordReq}
	rec := e.post(http.MethodPost, "/actions/credential/gcr-1/cancel", nil)
	if rec.Code != http.StatusOK || len(e.cred.calls) != 1 || e.cred.calls[0] != "cancel:gcr-1" {
		t.Errorf("status = %d calls = %v", rec.Code, e.cred.calls)
	}
}

func TestCredentialResolve_RejectsNonFormBody(t *testing.T) {
	e := newActionEnv(service.LifecycleReady)
	e.cred.pending = []service.CredentialView{passwordReq}

	req := httptest.NewRequest(http.MethodPost, "/actions/credential/gcr-1", strings.NewReader(`{"value":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.TokenHeader, testToken)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("JSON: status = %d, want 415", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/actions/credential/gcr-1", strings.NewReader("value=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(middleware.TokenHeader, testToken)
	rec = httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("不正なエンコード: status = %d, want 400", rec.Code)
	}
	if len(e.cred.calls) != 0 {
		t.Errorf("不正なリクエストで Resolve が呼ばれた: %v", e.cred.calls)
	}
}
