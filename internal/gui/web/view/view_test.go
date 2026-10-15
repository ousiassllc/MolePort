package view_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/atoms"
	"github.com/ousiassllc/moleport/internal/gui/web/view/layout"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
	"github.com/ousiassllc/moleport/internal/gui/web/view/pages"
	"github.com/ousiassllc/moleport/internal/i18n"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestStatusBadge_EscapesLabel(t *testing.T) {
	out := renderToString(t, atoms.StatusBadge("active", `<script>alert(1)</script>`))

	if strings.Contains(out, "<script>") {
		t.Errorf("ラベルがエスケープされていない: %s", out)
	}
	if !strings.Contains(out, `data-status="active"`) {
		t.Errorf("data-status がない: %s", out)
	}
}

func TestButton_VariantAndAttrs(t *testing.T) {
	out := renderToString(t, atoms.Button(atoms.ButtonProps{
		Label:   "Stop",
		Variant: atoms.ButtonDanger,
		Attrs:   templ.Attributes{"id": "btn-stop", "hx-post": "/actions/forwards/a/stop"},
	}))

	for _, want := range []string{`data-variant="danger"`, `id="btn-stop"`, `hx-post="/actions/forwards/a/stop"`, ">Stop</button>"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q が含まれない: %s", want, out)
		}
	}
}

func TestToast_RoleByKind(t *testing.T) {
	if out := renderToString(t, atoms.Toast(service.ToastError, "x")); !strings.Contains(out, `role="alert"`) {
		t.Errorf("error は role=alert: %s", out)
	}
	if out := renderToString(t, atoms.Toast(service.ToastInfo, "x")); !strings.Contains(out, `role="status"`) {
		t.Errorf("info は role=status: %s", out)
	}
}

func TestStatusBanner_LocalizedByLanguage(t *testing.T) {
	prev := i18n.CurrentLang()
	t.Cleanup(func() { _ = i18n.SetLang(prev) })

	for _, tt := range []struct {
		lang i18n.Lang
		want string
	}{
		{i18n.LangJA, "デーモンを起動しています"},
		{i18n.LangEN, "Starting the daemon"},
	} {
		if err := i18n.SetLang(tt.lang); err != nil {
			t.Fatalf("SetLang(%q): %v", tt.lang, err)
		}
		out := renderToString(t, organisms.StatusBanner(service.LifecycleStarting))
		if !strings.Contains(out, tt.want) {
			t.Errorf("lang=%s: %q が含まれない: %s", tt.lang, tt.want, out)
		}
	}
}

func TestStatusBanner_ReadyRendersNothing(t *testing.T) {
	if out := renderToString(t, organisms.StatusBanner(service.LifecycleReady)); out != "" {
		t.Errorf("ready は空であるべき: %q", out)
	}
}

func TestBaseLayout_EscapesTokenAndRendersChildren(t *testing.T) {
	c := layout.BaseLayout(layout.LayoutProps{Lang: "ja", Theme: "dark", Accent: "blue", Token: `"><script>x</script>`}) //nolint:gosec // XSS エスケープ検証用のダミー値
	var buf bytes.Buffer
	ctx := templ.WithChildren(context.Background(), atoms.StatusBadge("ok", "CHILD"))
	if err := c.Render(ctx, &buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "<script>x</script>") {
		t.Errorf("トークンがエスケープされていない: %s", out)
	}
	for _, want := range []string{`lang="ja"`, `data-theme="dark"`, `data-accent="blue"`, "CHILD", `id="modal-root"`, `id="toast-stack"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%q が含まれない: %s", want, out)
		}
	}
}

func TestBaseLayout_NoInlineScript(t *testing.T) {
	// CSP script-src 'self' のため、インラインスクリプト・インラインイベントハンドラーを出力してはならない。
	out := renderToString(t, pages.DashboardPage(layout.LayoutProps{Lang: "en", Theme: "system", Accent: "violet", Token: "t"}, service.LifecycleReady))

	for _, script := range strings.Split(out, "<script")[1:] {
		tag := script[:strings.Index(script, ">")]
		if !strings.Contains(tag, "src=") {
			t.Errorf("インライン <script> が含まれる: <script%s>", tag)
		}
	}
	if strings.Contains(out, " onclick=") || strings.Contains(out, " style=") {
		t.Errorf("インラインハンドラー/スタイルが含まれる: %s", out)
	}
}
