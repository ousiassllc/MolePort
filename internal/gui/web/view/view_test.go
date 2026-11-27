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
	"github.com/ousiassllc/moleport/internal/gui/web/view/molecules"
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
	for _, want := range []string{`lang="ja"`, `data-theme="dark"`, `data-accent="blue"`, "CHILD", `id="modal-root"`, `id="toast-stack"`, `id="toast-messages"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%q が含まれない: %s", want, out)
		}
	}
}

func TestBaseLayout_NoInlineScript(t *testing.T) {
	// CSP script-src 'self' のため、インラインスクリプト・インラインイベントハンドラーを出力してはならない。
	out := renderToString(t, pages.DashboardPage(pages.DashboardProps{
		Layout: layout.LayoutProps{Lang: "en", Theme: "system", Accent: "violet", Token: "t"},
		State:  service.LifecycleReady,
	}))

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

func TestElementID_EscapesNonAlphanumericWithoutCollision(t *testing.T) {
	tests := []struct{ name, want string }{
		{"web", "host-web"},
		{"a b", "host-a_20b"},
		{"a_b", "host-a_5fb"}, // "_" 自体もエスケープし、"a b" と衝突させない
		{"a-b", "host-a_2db"},
		{"日", "host-_e6_97_a5"},
		{"\x01", "host-_01"},
	}
	seen := map[string]string{}
	for _, tt := range tests {
		got := atoms.ElementID("host-", tt.name)
		if got != tt.want {
			t.Errorf("ElementID(%q) = %q, want %q", tt.name, got, tt.want)
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("ID 衝突: %q と %q がともに %q", prev, tt.name, got)
		}
		seen[got] = tt.name
	}
}

func TestToast_ErrorHasDismissButton(t *testing.T) {
	if out := renderToString(t, atoms.Toast(service.ToastError, "x")); !strings.Contains(out, "data-dismiss-toast") {
		t.Errorf("error トーストは手動で閉じるボタンを持つ: %s", out)
	}
	if out := renderToString(t, atoms.Toast(service.ToastSuccess, "x")); strings.Contains(out, "data-dismiss-toast") {
		t.Errorf("success トーストは自動消去のためボタンを持たない: %s", out)
	}
}

func TestDuration_EmptyRendersNothing(t *testing.T) {
	if out := renderToString(t, atoms.Duration("Uptime", "")); out != "" {
		t.Errorf("空の稼働時間は何も描画しない: %q", out)
	}
	if out := renderToString(t, atoms.Duration("Uptime", "5m 0s")); !strings.Contains(out, "5m 0s") {
		t.Errorf("稼働時間が含まれない: %q", out)
	}
}

func TestButton_TypeDefaultsToButtonAndAllowsSubmit(t *testing.T) {
	if out := renderToString(t, atoms.Button(atoms.ButtonProps{Label: "x", Variant: atoms.ButtonPrimary})); !strings.Contains(out, `type="button"`) {
		t.Errorf("既定は type=button: %s", out)
	}
	if out := renderToString(t, atoms.Button(atoms.ButtonProps{Label: "x", Variant: atoms.ButtonPrimary, Type: "submit"})); !strings.Contains(out, `type="submit"`) {
		t.Errorf("Type=submit は type=submit: %s", out)
	}
	if out := renderToString(t, atoms.Button(atoms.ButtonProps{Label: "x", Variant: atoms.ButtonPrimary, Type: "reset"})); !strings.Contains(out, `type="button"`) {
		t.Errorf("未知の Type は type=button にする: %s", out)
	}
}

func TestFormField_ErrorIsDescribedByInput(t *testing.T) {
	field := func(msg string) string {
		return renderToString(t, molecules.FormField(molecules.FormFieldProps{
			Label: "Port", Name: "local_port", Error: msg,
			Input: atoms.TextInput(atoms.TextInputProps{Name: "local_port", Invalid: msg != ""}),
		}))
	}

	out := field("bad port")
	for _, want := range []string{`aria-describedby="field-local_port-error"`, `id="field-local_port-error"`, `aria-invalid="true"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%q が含まれない: %s", want, out)
		}
	}
	if out := field(""); strings.Contains(out, "aria-describedby") || strings.Contains(out, "-error") || strings.Contains(out, "aria-invalid") {
		t.Errorf("エラーなしは aria-describedby もエラー要素も出さない: %s", out)
	}

	sel := renderToString(t, atoms.Select(atoms.SelectProps{Name: "language", Invalid: true}))
	if !strings.Contains(sel, `aria-describedby="field-language-error"`) || !strings.Contains(sel, `aria-invalid="true"`) {
		t.Errorf("Select も aria-describedby と aria-invalid=\"true\" を持つ: %s", sel)
	}
	if sel := renderToString(t, atoms.Select(atoms.SelectProps{Name: "language"})); strings.Contains(sel, "aria-describedby") || strings.Contains(sel, "aria-invalid") {
		t.Errorf("Invalid でない Select は aria-describedby / aria-invalid を出さない: %s", sel)
	}
}

func TestToastMessages_OOBOnlyWhenRequested(t *testing.T) {
	plain := renderToString(t, organisms.ToastMessages(false))
	for _, want := range []string{`id="toast-messages"`, `data-error-message="`, `data-dismiss-label="`, "hidden"} {
		if !strings.Contains(plain, want) {
			t.Errorf("%q が含まれない: %s", want, plain)
		}
	}
	if strings.Contains(plain, "hx-swap-oob") {
		t.Errorf("通常描画は OOB にしない: %s", plain)
	}
	if oob := renderToString(t, organisms.ToastMessages(true)); !strings.Contains(oob, `hx-swap-oob="true"`) {
		t.Errorf("oob=true は hx-swap-oob=\"true\" を出す: %s", oob)
	}
	if stack := renderToString(t, organisms.ToastStack()); strings.Contains(stack, "data-error-message") {
		t.Errorf("#toast-stack は文言属性を持たない: %s", stack)
	}
}

func TestSettingsForm_ThemeErrorDescribesFieldset(t *testing.T) {
	out := renderToString(t, organisms.SettingsForm(service.SettingsView{}, service.FormErrors{"theme_base": "bad base"}))
	for _, want := range []string{`id="settings-theme-base" aria-describedby="field-theme_base-error"`, `id="field-theme_base-error"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%q が含まれない: %s", want, out)
		}
	}
	if strings.Contains(out, "field-theme_accent-error") {
		t.Errorf("エラーのない accent には aria-describedby / エラー要素を出さない: %s", out)
	}
}

func TestOrganismButtons_RenderedByAtom(t *testing.T) {
	out := renderToString(t, organisms.SettingsForm(service.SettingsView{}, nil))
	for _, want := range []string{
		`id="settings-cancel"`, `data-close-modal`, `type="button"`,
		`id="settings-submit"`, `type="submit"`, `data-variant="primary"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%q が含まれない: %s", want, out)
		}
	}
}
