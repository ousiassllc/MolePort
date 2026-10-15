package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/credential"
	"github.com/ousiassllc/moleport/internal/gui/web/middleware"
)

// fakeForwards は service.Forwards のテスト用実装。
type fakeForwards struct {
	addName string
	addErrs service.FormErrors
	opErr   error
	lastAdd service.ForwardFormInput
	calls   []string
}

func (f *fakeForwards) HostNames() []string { return []string{"prod", "db"} }
func (f *fakeForwards) Form(host string) service.ForwardFormInput {
	return service.ForwardFormInput{Host: host, Type: "local"}
}

func (f *fakeForwards) Add(in service.ForwardFormInput) (string, service.FormErrors, error) {
	f.lastAdd = in
	f.calls = append(f.calls, "add")
	if f.opErr != nil || len(f.addErrs) > 0 {
		return "", f.addErrs, f.opErr
	}
	return f.addName, nil, nil
}

func (f *fakeForwards) op(kind string) func(string) error {
	return func(name string) error {
		f.calls = append(f.calls, kind+":"+name)
		return f.opErr
	}
}
func (f *fakeForwards) Start(name string) error  { return f.op("start")(name) }
func (f *fakeForwards) Stop(name string) error   { return f.op("stop")(name) }
func (f *fakeForwards) Delete(name string) error { return f.op("delete")(name) }

// fakeHosts は service.Hosts のテスト用実装。
type fakeHosts struct {
	opErr error
	calls []string
}

func (f *fakeHosts) Connect(name string) error {
	f.calls = append(f.calls, "connect:"+name)
	return f.opErr
}

func (f *fakeHosts) Disconnect(name string) error {
	f.calls = append(f.calls, "disconnect:"+name)
	return f.opErr
}

type actionEnv struct {
	h     http.Handler
	fwd   *fakeForwards
	hosts *fakeHosts
	cred  *fakePrompter
}

func newActionEnv(state service.LifecycleState) *actionEnv {
	e := &actionEnv{fwd: &fakeForwards{addName: "prod-local-8080"}, hosts: &fakeHosts{}, cred: &fakePrompter{}}
	e.h = NewRouter(Deps{
		Life:      fakeLife{state: state},
		Dashboard: sampleDashboard(),
		Toasts:    service.NewNotificationQueue(),
		Forwards:  e.fwd,
		Hosts:     e.hosts,
		Cred:      e.cred,
		Token:     testToken,
		Static:    fstest.MapFS{"dist/.gitkeep": {}},
	})
	return e
}

// post は トークン付きのフォーム POST（HTMX 相当）を送る。
func (e *actionEnv) post(method, target string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(middleware.TokenHeader, testToken)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

const oobToast = `hx-swap-oob="beforeend:#toast-stack"`

var _ credential.Prompter = (*fakePrompter)(nil)

func TestActions_RequireToken(t *testing.T) {
	e := newActionEnv(service.LifecycleReady)
	for _, tt := range []struct{ method, target string }{
		{http.MethodPost, "/actions/forwards"},
		{http.MethodPost, "/actions/forwards/web/start"},
		{http.MethodPost, "/actions/forwards/web/stop"},
		{http.MethodDelete, "/actions/forwards/web"},
		{http.MethodPost, "/actions/hosts/prod/connect"},
		{http.MethodPost, "/actions/hosts/prod/disconnect"},
		{http.MethodPost, "/actions/credential/gcr-1"},
		{http.MethodPost, "/actions/credential/gcr-1/cancel"},
	} {
		if got := serve(e.h, tt.method, tt.target, nil).Code; got != http.StatusForbidden {
			t.Errorf("%s %s トークンなし: status = %d, want 403", tt.method, tt.target, got)
		}
	}
	if len(e.fwd.calls)+len(e.hosts.calls)+len(e.cred.calls) != 0 {
		t.Error("トークンなしで Service が呼ばれた")
	}
}

func TestForwardAdd_SuccessReturnsPanelToastAndClosesModal(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleReady)
	rec := e.post(http.MethodPost, "/actions/forwards", url.Values{
		"host": {"prod"}, "type": {"remote"}, "name": {""}, "local_port": {"8080"},
		"remote_host": {"db"}, "remote_port": {"5432"}, "remote_bind_addr": {"0.0.0.0"}, "auto_connect": {"on"},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := service.ForwardFormInput{Host: "prod", Type: "remote", LocalPort: "8080", RemoteHost: "db", RemotePort: "5432", RemoteBindAddr: "0.0.0.0", AutoConnect: true}
	if e.fwd.lastAdd != want {
		t.Errorf("入力 = %+v, want %+v", e.fwd.lastAdd, want)
	}
	if got := rec.Header().Get("HX-Trigger"); !strings.Contains(got, "mp:close-modal") {
		t.Errorf("HX-Trigger = %q, want mp:close-modal", got)
	}
	body := rec.Body.String()
	for _, s := range []string{`id="fwd-web"`, oobToast, `Added forward rule &#34;prod-local-8080&#34;.`} {
		if !strings.Contains(body, s) {
			t.Errorf("本文に %q が無い:\n%s", s, body)
		}
	}
}

func TestForwardAdd_ValidationErrorRerendersFormWith422(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleReady)
	e.fwd.addErrs = service.FormErrors{"local_port": "bad port"}
	rec := e.post(http.MethodPost, "/actions/forwards", url.Values{
		"host": {"prod"}, "type": {"local"}, "name": {`<script>x</script>`}, "local_port": {"99999"},
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := rec.Header().Get("HX-Retarget"); got != "#modal-root" {
		t.Errorf("HX-Retarget = %q, want #modal-root", got)
	}
	body := rec.Body.String()
	for _, s := range []string{`id="forward-form"`, "bad port", `value="99999"`, `aria-invalid`, `&lt;script&gt;`} {
		if !strings.Contains(body, s) {
			t.Errorf("本文に %q が無い:\n%s", s, body)
		}
	}
	if strings.Contains(body, "<script>x") {
		t.Error("入力値がエスケープされていない")
	}
	if strings.Contains(body, `id="fwd-web"`) {
		t.Error("422 で転送パネルを返した")
	}
}

func TestForwardTransitions(t *testing.T) {
	setEnglish(t)
	tests := []struct {
		name, method, target string
		err                  error
		status               int
		trigger              string
		wantCall             string
	}{
		{"開始", http.MethodPost, "/actions/forwards/a%20b/start", nil, http.StatusOK, "", "start:a b"},
		{"停止", http.MethodPost, "/actions/forwards/web/stop", nil, http.StatusOK, "", "stop:web"},
		{"削除", http.MethodDelete, "/actions/forwards/web", nil, http.StatusOK, "mp:close-modal", "delete:web"},
		{"状態不一致は 409", http.MethodPost, "/actions/forwards/web/start", service.ErrStateConflict, http.StatusConflict, "mp:forwards", "start:web"},
		{"ルールなしは 404", http.MethodPost, "/actions/forwards/web/stop", &core.NotFoundError{Resource: "rule", Name: "web"}, http.StatusNotFound, "mp:forwards", "stop:web"},
		{"起動中は 409", http.MethodDelete, "/actions/forwards/web", service.ErrNotReady, http.StatusConflict, "mp:forwards", "delete:web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newActionEnv(service.LifecycleReady)
			e.fwd.opErr = tt.err
			rec := e.post(tt.method, tt.target, nil)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if len(e.fwd.calls) != 1 || e.fwd.calls[0] != tt.wantCall {
				t.Errorf("呼び出し = %v, want [%s]", e.fwd.calls, tt.wantCall)
			}
			if tt.trigger != "" && !strings.Contains(rec.Header().Get("HX-Trigger"), tt.trigger) {
				t.Errorf("HX-Trigger = %q, want %s", rec.Header().Get("HX-Trigger"), tt.trigger)
			}
			body := rec.Body.String()
			if tt.err == nil {
				if !strings.Contains(body, `id="fwd-web"`) {
					t.Errorf("成功時に転送パネルを返さない:\n%s", body)
				}
				return
			}
			// 失敗時は OOB トーストのみでメイン領域を変更しない。
			if rec.Header().Get("HX-Reswap") != "none" || !strings.Contains(body, oobToast) || strings.Contains(body, `id="fwd-web"`) {
				t.Errorf("失敗時の応答: HX-Reswap=%q body=%s", rec.Header().Get("HX-Reswap"), body)
			}
		})
	}
}

func TestHostTransitions(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleReady)
	rec := e.post(http.MethodPost, "/actions/hosts/prod%20web/connect", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `data-name="prod web"`) {
		t.Fatalf("接続: status = %d body=%s", rec.Code, rec.Body.String())
	}
	if e.hosts.calls[0] != "connect:prod web" {
		t.Errorf("呼び出し = %v", e.hosts.calls)
	}

	e.hosts.opErr = service.ErrStateConflict
	rec = e.post(http.MethodPost, "/actions/hosts/db/disconnect", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Header().Get("HX-Trigger"), "mp:hosts") {
		t.Errorf("409: status = %d HX-Trigger = %q", rec.Code, rec.Header().Get("HX-Trigger"))
	}
	if !strings.Contains(rec.Body.String(), `Host &#34;db&#34; cannot be changed in its current state.`) {
		t.Errorf("409 のトースト:\n%s", rec.Body.String())
	}
}

func TestForwardFormFragment_FieldsByType(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleReady)
	tests := []struct {
		query       string
		want, avoid []string
	}{
		{"?host=db", []string{`<option value="db" selected`, `name="remote_host"`, `name="remote_port"`}, []string{`name="remote_bind_addr"`}},
		{"?type=remote&host=prod&local_port=80", []string{`name="remote_bind_addr"`, `name="remote_host"`, `value="80"`}, nil},
		{"?type=dynamic&host=prod", nil, []string{`name="remote_host"`, `name="remote_port"`, `name="remote_bind_addr"`}},
	}
	for _, tt := range tests {
		rec := serve(e.h, http.MethodGet, "/fragments/forward-form"+tt.query, hxHeader)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", tt.query, rec.Code)
		}
		body := rec.Body.String()
		for _, s := range tt.want {
			if !strings.Contains(body, s) {
				t.Errorf("%s: 本文に %q が無い", tt.query, s)
			}
		}
		for _, s := range tt.avoid {
			if strings.Contains(body, s) {
				t.Errorf("%s: 本文に %q がある", tt.query, s)
			}
		}
	}
}

func TestForwardDeleteFragment(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleReady)
	rec := serve(e.h, http.MethodGet, "/fragments/forward-delete?name=a%20b", hxHeader)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `hx-delete="/actions/forwards/a%20b"`) {
		t.Errorf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := serve(e.h, http.MethodGet, "/fragments/forward-delete", hxHeader).Code; got != http.StatusBadRequest {
		t.Errorf("name なし: status = %d, want 400", got)
	}
}

func TestFormFragments_NotReady(t *testing.T) {
	setEnglish(t)
	e := newActionEnv(service.LifecycleStarting)
	for _, target := range []string{"/fragments/forward-form", "/fragments/forward-delete?name=web"} {
		rec := serve(e.h, http.MethodGet, target, hxHeader)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "The daemon is still starting.") {
			t.Errorf("%s: status = %d body=%s", target, rec.Code, rec.Body.String())
		}
	}
}
