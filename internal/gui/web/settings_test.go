package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ousiassllc/moleport/internal/core/guiconfig"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/settings"
)

// fakeSettings は settings.Settings のテスト用実装。
type fakeSettings struct {
	notReady bool
	view     service.SettingsView
	errs     service.FormErrors
	saved    []service.SettingsView
	openErr  error
	opens    int
}

func (f *fakeSettings) View() (service.SettingsView, error) {
	if f.notReady {
		return service.SettingsView{}, service.ErrNotReady
	}
	return f.view, nil
}

func (f *fakeSettings) Save(in service.SettingsView) (service.FormErrors, error) {
	if f.notReady {
		return nil, service.ErrNotReady
	}
	if len(f.errs) > 0 {
		return f.errs, nil
	}
	f.saved = append(f.saved, in)
	f.view = in
	return nil, nil
}

func (f *fakeSettings) OpenUpdate() error {
	f.opens++
	return f.openErr
}

var _ settings.Settings = (*fakeSettings)(nil)

func settingsRouter(set *fakeSettings, theme guiconfig.Theme) http.Handler {
	return NewRouter(Deps{
		Life:      fakeLife{state: service.LifecycleReady},
		Dashboard: sampleDashboard(),
		Toasts:    service.NewNotificationQueue(),
		Settings:  set,
		Theme:     theme,
		Token:     testToken,
		Static:    fstest.MapFS{"dist/.gitkeep": {}},
	})
}

func savedSettings() *fakeSettings {
	return &fakeSettings{view: service.SettingsView{Language: "en", ThemeBase: "dark", ThemeAccent: "cyan", UpdateCheckEnabled: true}}
}

func TestIndex_UsesSavedTheme(t *testing.T) {
	setEnglish(t)
	body := serve(settingsRouter(savedSettings(), guiconfig.Default().Theme), http.MethodGet, "/", nil).Body.String()
	if !strings.Contains(body, `data-theme="dark" data-accent="cyan"`) {
		t.Errorf("保存値のテーマが <html> に出ない:\n%s", body[:min(len(body), 300)])
	}

	// 内蔵デーモンの起動前は起動時に読んだテーマを使う。
	body = serve(settingsRouter(&fakeSettings{notReady: true}, guiconfig.Theme{Base: "light", Accent: "orange"}), http.MethodGet, "/", nil).Body.String()
	if !strings.Contains(body, `data-theme="light" data-accent="orange"`) {
		t.Errorf("起動時のテーマが <html> に出ない:\n%s", body[:min(len(body), 300)])
	}
}

func TestSettingsFragment(t *testing.T) {
	setEnglish(t)
	rec := serve(settingsRouter(savedSettings(), guiconfig.Theme{}), http.MethodGet, "/fragments/settings", hxHeader)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, s := range []string{
		`id="settings-form"`, `hx-post="/actions/settings"`,
		`<option value="en" selected`, `<option value="ja"`,
		`name="theme_base" value="dark" checked`, `name="theme_base" value="system"`, `name="theme_base" value="light"`,
		`name="theme_accent" value="cyan" checked`, `value="violet"`, `value="blue"`, `value="green"`, `value="orange"`,
		`data-theme-preview`, `name="update_check_enabled" checked`, `data-close-modal`,
	} {
		if !strings.Contains(body, s) {
			t.Errorf("本文に %q が無い", s)
		}
	}

	rec = serve(settingsRouter(&fakeSettings{notReady: true}, guiconfig.Theme{}), http.MethodGet, "/fragments/settings", hxHeader)
	if rec.Code != http.StatusConflict {
		t.Errorf("起動中: status = %d, want 409", rec.Code)
	}
}

func TestSettingsSave_Success(t *testing.T) {
	setEnglish(t)
	set := savedSettings()
	e := &actionEnv{h: settingsRouter(set, guiconfig.Theme{})}
	rec := e.post(http.MethodPost, "/actions/settings", url.Values{
		"language": {"ja"}, "theme_base": {"light"}, "theme_accent": {"orange"},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := service.SettingsView{Language: "ja", ThemeBase: "light", ThemeAccent: "orange", UpdateCheckEnabled: false}
	if len(set.saved) != 1 || set.saved[0] != want {
		t.Errorf("保存 = %+v, want %+v", set.saved, want)
	}
	var trigger map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &trigger); err != nil {
		t.Fatalf("HX-Trigger = %q: %v", rec.Header().Get("HX-Trigger"), err)
	}
	for _, ev := range []string{"mp:close-modal", "mp:all"} {
		if _, ok := trigger[ev]; !ok {
			t.Errorf("HX-Trigger に %s が無い: %v", ev, trigger)
		}
	}
	if got := string(trigger["mp:settings-applied"]); got != `{"accent":"orange","lang":"ja","theme":"light"}` {
		t.Errorf("mp:settings-applied = %s", got)
	}
	if rec.Header().Get("HX-Refresh") != "" {
		t.Error("HX-Refresh を使ってはならない")
	}
	if !strings.Contains(rec.Body.String(), oobToast) {
		t.Errorf("OOB トーストが無い: %s", rec.Body.String())
	}
}

func TestSettingsSave_ValidationRerendersWith422(t *testing.T) {
	setEnglish(t)
	set := savedSettings()
	set.errs = service.FormErrors{"theme_accent": "pick one"}
	e := &actionEnv{h: settingsRouter(set, guiconfig.Theme{})}
	rec := e.post(http.MethodPost, "/actions/settings", url.Values{
		"language": {"en"}, "theme_base": {"light"}, "theme_accent": {"red"}, "update_check_enabled": {"on"},
	})

	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#modal-root" {
		t.Fatalf("status = %d HX-Retarget = %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	body := rec.Body.String()
	// 入力値を保持して再描画する。
	for _, s := range []string{"pick one", `name="theme_base" value="light" checked`, `name="update_check_enabled" checked`} {
		if !strings.Contains(body, s) {
			t.Errorf("本文に %q が無い", s)
		}
	}
}

func TestSettingsSave_RequiresToken(t *testing.T) {
	set := savedSettings()
	h := settingsRouter(set, guiconfig.Theme{})
	for _, target := range []string{"/actions/settings", "/actions/update/open"} {
		if got := serve(h, http.MethodPost, target, nil).Code; got != http.StatusForbidden {
			t.Errorf("%s トークンなし: status = %d, want 403", target, got)
		}
	}
	if len(set.saved) != 0 || set.opens != 0 {
		t.Error("トークンなしで Service が呼ばれた")
	}
}

func TestUpdateOpen(t *testing.T) {
	setEnglish(t)
	tests := []struct {
		name    string
		err     error
		status  int
		trigger string
	}{
		{"成功は 204", nil, http.StatusNoContent, ""},
		{"更新なしは 409", settings.ErrNoUpdate, http.StatusConflict, "mp:header"},
		{"内部エラーは 500", errors.New("boom"), http.StatusInternalServerError, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := savedSettings()
			set.openErr = tt.err
			e := &actionEnv{h: settingsRouter(set, guiconfig.Theme{})}
			// URL をクライアントから送っても使わない（Service はキャッシュから取得する）。
			rec := e.post(http.MethodPost, "/actions/update/open", url.Values{"url": {"https://evil.example/"}})
			if rec.Code != tt.status || set.opens != 1 {
				t.Fatalf("status = %d opens = %d", rec.Code, set.opens)
			}
			if tt.trigger != "" && !strings.Contains(rec.Header().Get("HX-Trigger"), tt.trigger) {
				t.Errorf("HX-Trigger = %q", rec.Header().Get("HX-Trigger"))
			}
			if tt.status == http.StatusConflict && !strings.Contains(rec.Body.String(), "No new version is available.") {
				t.Errorf("409 のトースト: %s", rec.Body.String())
			}
		})
	}
}

func TestHeaderFragment_UpdateBadgeAndSettingsButton(t *testing.T) {
	setEnglish(t)
	d := sampleDashboard()
	h := dashboardRouter(service.LifecycleReady, d, service.NewNotificationQueue())
	body := serve(h, http.MethodGet, "/fragments/header", hxHeader).Body.String()
	if strings.Contains(body, "/actions/update/open") {
		t.Error("更新なしでバッジを描画した")
	}
	if !strings.Contains(body, `hx-get="/fragments/settings"`) {
		t.Errorf("設定ボタンが無い: %s", body)
	}

	d.header.UpdateAvailable, d.header.LatestVersion, d.header.ReleaseURL = true, "v9.0.0", "https://github.com/ousiassllc/moleport/releases/tag/v9.0.0"
	body = serve(h, http.MethodGet, "/fragments/header", hxHeader).Body.String()
	if !strings.Contains(body, `hx-post="/actions/update/open"`) || !strings.Contains(body, "v9.0.0 available") {
		t.Errorf("更新バッジが無い: %s", body)
	}
	if strings.Contains(body, "releases/tag") {
		t.Error("リリース URL をクライアントに渡してはならない")
	}
}
