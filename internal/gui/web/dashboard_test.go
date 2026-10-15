package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// fakeDashboard は固定の ViewModel を返す service.Dashboard のテスト用実装。
type fakeDashboard struct {
	hosts    []service.HostView
	forwards []service.ForwardView
	header   service.HeaderView
	status   service.StatusView
}

func (f *fakeDashboard) Header() service.HeaderView      { return f.header }
func (f *fakeDashboard) Hosts() []service.HostView       { return f.hosts }
func (f *fakeDashboard) Forwards() []service.ForwardView { return f.forwards }
func (f *fakeDashboard) Status() service.StatusView      { return f.status }

var hxHeader = map[string]string{"HX-Request": "true"}

func dashboardRouter(state service.LifecycleState, d *fakeDashboard, q *service.NotificationQueue) http.Handler {
	return NewRouter(Deps{
		Life:      fakeLife{state: state},
		Dashboard: d,
		Toasts:    q,
		Token:     testToken,
		Static:    fstest.MapFS{"dist/.gitkeep": {}},
	})
}

func sampleDashboard() *fakeDashboard {
	return &fakeDashboard{
		header: service.HeaderView{Version: "1.2.3", PID: 4242},
		hosts: []service.HostView{
			{Name: "prod web", Address: "alice@10.0.0.1:22", State: "disconnected", StateLabel: "Disconnected",
				Actions: service.HostActions{CanConnect: true, CanAddForward: true}},
			{Name: "db", Address: "db:22", State: "connected", StateLabel: "Connected", ActiveCount: 2,
				Actions: service.HostActions{CanDisconnect: true, CanAddForward: true}},
		},
		forwards: []service.ForwardView{
			{Name: "web", Host: "db", Type: "local", Route: ":8080 → localhost:80", Status: "active", StatusLabel: "Active",
				Uptime: "2h 15m", Sent: "1.2MB", Received: "3.4KB", Reconnects: 3,
				Actions: service.ForwardActions{CanStop: true, CanDelete: true}},
			{Name: "bad", Host: "db", Type: "local", Route: ":9090 → localhost:90", Status: "error", StatusLabel: "Error",
				LastError: "bind: <b>in use</b>",
				Actions:   service.ForwardActions{CanStart: true, CanDelete: true}},
		},
		status: service.StatusView{Hosts: 2, ConnectedHosts: 1, Forwards: 2, ActiveForwards: 1},
	}
}

func TestPage_Ready_RendersAllRegionsInline(t *testing.T) {
	setEnglish(t)
	body := serve(dashboardRouter(service.LifecycleReady, sampleDashboard(), service.NewNotificationQueue()), http.MethodGet, "/", nil).Body.String()

	for _, want := range []string{
		`id="header"`, `hx-get="/fragments/header"`, `mp:header from:body`,
		`id="hosts-panel"`, `hx-get="/fragments/hosts"`, `mp:hosts from:body, mp:all from:body`,
		`id="forwards-panel"`, `hx-get="/fragments/forwards"`, `mp:forwards from:body`,
		`id="status-bar"`, `hx-get="/fragments/status"`, `mp:status from:body`,
		`hx-get="/fragments/toasts"`, `mp:toast from:body`,
		"1.2.3", "4242", "db", "web", "2h 15m", "1.2MB", "3.4KB",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("本文に %q が含まれない:\n%s", want, body)
		}
	}
}

func TestPage_NotReady_RendersNoListOrActions(t *testing.T) {
	for _, state := range []service.LifecycleState{service.LifecycleStarting, service.LifecycleFailed} {
		body := serve(dashboardRouter(state, sampleDashboard(), service.NewNotificationQueue()), http.MethodGet, "/", nil).Body.String()

		for _, unwanted := range []string{"hx-post=", `class="mp-row"`, "db"} {
			if strings.Contains(body, unwanted) {
				t.Errorf("%s: ready でない間は %q を描画してはならない", state, unwanted)
			}
		}
	}
}

func TestFragments_HostsAndForwards_StableIDsAndActions(t *testing.T) {
	setEnglish(t)
	h := dashboardRouter(service.LifecycleReady, sampleDashboard(), service.NewNotificationQueue())

	hosts := serve(h, http.MethodGet, "/fragments/hosts", hxHeader)
	if hosts.Code != http.StatusOK {
		t.Fatalf("hosts: status = %d", hosts.Code)
	}
	hb := hosts.Body.String()
	if strings.Contains(strings.ToLower(hb), "<html") {
		t.Errorf("フラグメントに <html> を含めてはならない: %s", hb)
	}
	for _, want := range []string{
		// 英数字以外は _ + 16 進（空白 = 0x20）でエスケープし、元の名前は data-name に保持する。
		`id="host-prod_20web"`, `data-name="prod web"`,
		`id="host-prod_20web-connect"`, `hx-post="/actions/hosts/prod%20web/connect"`,
		`id="host-db-disconnect"`, `hx-post="/actions/hosts/db/disconnect"`,
		`id="host-db-add-forward"`,
	} {
		if !strings.Contains(hb, want) {
			t.Errorf("hosts に %q が含まれない:\n%s", want, hb)
		}
	}
	// 実行可能なアクションだけを描画する。
	for _, unwanted := range []string{`id="host-prod_20web-disconnect"`, `id="host-db-connect"`} {
		if strings.Contains(hb, unwanted) {
			t.Errorf("hosts に実行不可のアクション %q を描画してはならない", unwanted)
		}
	}

	fb := serve(h, http.MethodGet, "/fragments/forwards", hxHeader).Body.String()
	for _, want := range []string{
		`id="fwd-web"`, `data-name="web"`, `id="fwd-web-stop"`, `id="fwd-web-delete"`,
		`id="fwd-bad-start"`, `hx-post="/actions/forwards/bad/start"`,
		"2h 15m", "1.2MB", "3.4KB",
		"Reconnects: 3",
		// エラー詳細は HTML エスケープされる。
		"bind: &lt;b&gt;in use&lt;/b&gt;",
	} {
		if !strings.Contains(fb, want) {
			t.Errorf("forwards に %q が含まれない:\n%s", want, fb)
		}
	}
	for _, unwanted := range []string{`id="fwd-web-start"`, `id="fwd-bad-stop"`, "<b>in use</b>"} {
		if strings.Contains(fb, unwanted) {
			t.Errorf("forwards に %q を含めてはならない", unwanted)
		}
	}
}

func TestFragments_EmptyListsShowGuidance(t *testing.T) {
	setEnglish(t)
	h := dashboardRouter(service.LifecycleReady, &fakeDashboard{}, service.NewNotificationQueue())

	if body := serve(h, http.MethodGet, "/fragments/hosts", hxHeader).Body.String(); !strings.Contains(body, "No hosts in the SSH config.") {
		t.Errorf("ホスト 0 件の案内がない: %s", body)
	}
	if body := serve(h, http.MethodGet, "/fragments/forwards", hxHeader).Body.String(); !strings.Contains(body, "No forward rules yet.") {
		t.Errorf("転送 0 件の案内がない: %s", body)
	}
}

func TestFragments_HeaderAndStatus(t *testing.T) {
	setEnglish(t)
	h := dashboardRouter(service.LifecycleReady, sampleDashboard(), service.NewNotificationQueue())

	header := serve(h, http.MethodGet, "/fragments/header", hxHeader).Body.String()
	for _, want := range []string{"MolePort", "Version: 1.2.3", "PID: 4242"} {
		if !strings.Contains(header, want) {
			t.Errorf("header に %q が含まれない:\n%s", want, header)
		}
	}
	status := serve(h, http.MethodGet, "/fragments/status", hxHeader).Body.String()
	for _, want := range []string{"Hosts: 2", "Connected: 1", "Forwards: 2", "Active: 1"} {
		if !strings.Contains(status, want) {
			t.Errorf("status に %q が含まれない:\n%s", want, status)
		}
	}
}

func TestFragments_Toasts_DrainsQueue(t *testing.T) {
	q := service.NewNotificationQueue()
	h := dashboardRouter(service.LifecycleReady, &fakeDashboard{}, q)

	if got := serve(h, http.MethodGet, "/fragments/toasts", hxHeader); got.Code != http.StatusNoContent || got.Body.Len() != 0 {
		t.Errorf("空のキュー: status = %d, body = %q, want 204 + 空", got.Code, got.Body.String())
	}

	q.Push(service.ToastView{Kind: service.ToastError, Message: "boom <i>x</i>"})
	q.Push(service.ToastView{Kind: service.ToastInfo, Message: "hello"})
	rec := serve(h, http.MethodGet, "/fragments/toasts", hxHeader)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for _, want := range []string{`hx-swap-oob="beforeend:#toast-stack"`, `role="alert"`, `role="status"`, "boom &lt;i&gt;x&lt;/i&gt;", "hello"} {
		if !strings.Contains(body, want) {
			t.Errorf("toasts に %q が含まれない:\n%s", want, body)
		}
	}
	if strings.Index(body, "boom") > strings.Index(body, "hello") {
		t.Error("通知は古い順（FIFO）に出力される")
	}

	// 取り出し済みのため 2 回目は空。
	if got := serve(h, http.MethodGet, "/fragments/toasts", hxHeader).Code; got != http.StatusNoContent {
		t.Errorf("2 回目: status = %d, want 204", got)
	}
}

func TestFragments_RequireHXRequest(t *testing.T) {
	h := dashboardRouter(service.LifecycleReady, sampleDashboard(), service.NewNotificationQueue())
	for _, name := range []string{"header", "hosts", "forwards", "status", "toasts"} {
		req := httptest.NewRequest(http.MethodGet, "/fragments/"+name, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("/fragments/%s HX-Request なし: status = %d, want 400", name, rec.Code)
		}
	}
}

func setEnglish(t *testing.T) {
	t.Helper()
	prev := i18n.CurrentLang()
	t.Cleanup(func() { _ = i18n.SetLang(prev) })
	if err := i18n.SetLang(i18n.LangEN); err != nil {
		t.Fatalf("SetLang: %v", err)
	}
}
