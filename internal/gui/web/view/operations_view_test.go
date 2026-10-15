package view_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/molecules"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
)

// HATEOAS: Actions のうち実行可能なものだけを描画する。
func TestForwardRow_RendersOnlyAllowedActions(t *testing.T) {
	tests := []struct {
		name        string
		actions     service.ForwardActions
		want, avoid []string
	}{
		{"停止中", service.ForwardActions{CanStart: true, CanDelete: true},
			[]string{`id="fwd-web-start"`, `id="fwd-web-delete"`}, []string{`id="fwd-web-stop"`}},
		{"稼働中", service.ForwardActions{CanStop: true, CanDelete: true},
			[]string{`id="fwd-web-stop"`, `id="fwd-web-delete"`}, []string{`id="fwd-web-start"`}},
		{"操作なし", service.ForwardActions{},
			nil, []string{`id="fwd-web-start"`, `id="fwd-web-stop"`, `id="fwd-web-delete"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderToString(t, molecules.ForwardRow(service.ForwardView{Name: "web", Actions: tt.actions}))
			for _, s := range tt.want {
				if !strings.Contains(out, s) {
					t.Errorf("%q が無い: %s", s, out)
				}
			}
			for _, s := range tt.avoid {
				if strings.Contains(out, s) {
					t.Errorf("%q がある: %s", s, out)
				}
			}
		})
	}
}

func TestHostRow_RendersOnlyAllowedActions(t *testing.T) {
	out := renderToString(t, molecules.HostRow(service.HostView{Name: "prod", Actions: service.HostActions{CanDisconnect: true}}))
	if !strings.Contains(out, `hx-post="/actions/hosts/prod/disconnect"`) || strings.Contains(out, "/connect") || strings.Contains(out, "forward-form") {
		t.Errorf("切断のみ描画されるべき: %s", out)
	}
}

func TestForwardForm_KeepsInputAndShowsErrors(t *testing.T) {
	out := renderToString(t, organisms.ForwardForm(
		service.ForwardFormInput{Host: "db", Type: "remote", Name: `"><b>x`, LocalPort: "8080"},
		service.FormErrors{"local_port": "ポートが不正", "_form": "保存に失敗"},
		[]string{"prod", "db"},
	))
	for _, s := range []string{`<option value="db" selected`, `name="remote_bind_addr"`, `value="8080"`, "ポートが不正", "保存に失敗", `&#34;&gt;&lt;b&gt;x`} {
		if !strings.Contains(out, s) {
			t.Errorf("%q が無い: %s", s, out)
		}
	}
	if strings.Contains(out, "<b>x") {
		t.Errorf("入力値がエスケープされていない: %s", out)
	}
}

func TestCredentialPrompt_EchoSelectsInputType(t *testing.T) {
	if out := renderToString(t, molecules.CredentialPrompt(molecules.CredentialPromptProps{Label: "Code", Name: "answers[]", Echo: true})); !strings.Contains(out, `type="text"`) {
		t.Errorf("Echo=true は text: %s", out)
	}
	out := renderToString(t, molecules.CredentialPrompt(molecules.CredentialPromptProps{Label: "Password", Name: "value"}))
	if !strings.Contains(out, `type="password"`) || !strings.Contains(out, `autocomplete="off"`) || strings.Contains(out, "value=\"") {
		t.Errorf("Echo=false は password で値を持たない: %s", out)
	}
}

func TestForwardDeleteDialog_EscapesNameInPath(t *testing.T) {
	out := renderToString(t, organisms.ForwardDeleteDialog("a/b c"))
	if !strings.Contains(out, `hx-delete="/actions/forwards/a%2Fb%20c"`) || !strings.Contains(out, "data-close-modal") {
		t.Errorf("削除確認: %s", out)
	}
}
