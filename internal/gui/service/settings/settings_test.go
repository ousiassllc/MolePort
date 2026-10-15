package settings

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/guiconfig"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/servicetest"
	"github.com/ousiassllc/moleport/internal/i18n"
)

type staticBackend struct {
	b  service.Backend
	ok bool
}

func (s staticBackend) Backend() (service.Backend, bool) { return s.b, s.ok }

type fakeOpener struct{ urls []string }

func (o *fakeOpener) OpenURL(url string) { o.urls = append(o.urls, url) }

func keepLang(t *testing.T, lang i18n.Lang) {
	t.Helper()
	prev := i18n.CurrentLang()
	t.Cleanup(func() { _ = i18n.SetLang(prev) })
	if err := i18n.SetLang(lang); err != nil {
		t.Fatal(err)
	}
}

type env struct {
	cfg  *servicetest.Config
	ver  *servicetest.Version
	open *fakeOpener
	s    *Service
}

func newEnv(cfg core.Config, ver *servicetest.Version) *env {
	e := &env{cfg: servicetest.NewConfig(cfg), ver: ver, open: &fakeOpener{}}
	b := service.Backend{Config: e.cfg}
	if ver != nil {
		b.Version = ver
	}
	e.s = New(staticBackend{b: b, ok: true}, e.open)
	return e
}

func baseConfig() core.Config {
	cfg := core.DefaultConfig()
	cfg.TUI.Theme = core.ThemeConfig{Base: "light", Accent: "green"}
	return cfg
}

func TestService_View(t *testing.T) {
	keepLang(t, i18n.LangJA)
	cfg := baseConfig()
	cfg.GUI.Theme = guiconfig.Theme{Base: "dark", Accent: "cyan"}
	cfg.UpdateCheck.Enabled = false

	got, err := newEnv(cfg, nil).s.View()
	if err != nil {
		t.Fatal(err)
	}
	want := service.SettingsView{Language: "ja", ThemeBase: "dark", ThemeAccent: "cyan"}
	if got != want {
		t.Errorf("View = %+v, want %+v", got, want)
	}
}

func TestService_SaveAppliesOnlyOwnedKeys(t *testing.T) {
	keepLang(t, i18n.LangJA)
	ver := &servicetest.Version{}
	e := newEnv(baseConfig(), ver)

	errs, err := e.s.Save(service.SettingsView{Language: "en", ThemeBase: "light", ThemeAccent: "orange", UpdateCheckEnabled: false})
	if err != nil || len(errs) != 0 {
		t.Fatalf("Save: errs=%v err=%v", errs, err)
	}
	saved := e.cfg.GetConfig()
	if saved.Language != "en" || saved.GUI.Theme != (guiconfig.Theme{Base: "light", Accent: "orange"}) || saved.UpdateCheck.Enabled {
		t.Errorf("保存値 = %+v", saved)
	}
	if saved.TUI.Theme != (core.ThemeConfig{Base: "light", Accent: "green"}) {
		t.Errorf("tui.theme が変更された: %+v", saved.TUI.Theme)
	}
	if saved.UpdateCheck.Interval != baseConfig().UpdateCheck.Interval {
		t.Error("update_check.interval が変更された")
	}
	if i18n.CurrentLang() != i18n.LangEN {
		t.Errorf("表示言語 = %s, want en（即時反映）", i18n.CurrentLang())
	}
	if got := ver.EnabledCalls(); !reflect.DeepEqual(got, []bool{false}) {
		t.Errorf("SetEnabled = %v, want [false]", got)
	}
}

func TestService_SaveValidation(t *testing.T) {
	keepLang(t, i18n.LangEN)
	e := newEnv(baseConfig(), nil)
	errs, err := e.s.Save(service.SettingsView{Language: "fr", ThemeBase: "sepia", ThemeAccent: "red"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"language", "theme_base", "theme_accent"} {
		if errs[f] == "" {
			t.Errorf("errs[%s] が空（errs=%v）", f, errs)
		}
	}
	if e.cfg.Saves() != 0 {
		t.Error("検証エラーなのに保存した")
	}
}

func TestService_SaveFailureKeepsLanguage(t *testing.T) {
	keepLang(t, i18n.LangJA)
	e := newEnv(baseConfig(), nil)
	e.cfg.Err = errors.New("disk full")
	if _, err := e.s.Save(service.SettingsView{Language: "en", ThemeBase: "dark", ThemeAccent: "blue"}); err == nil {
		t.Fatal("保存失敗がエラーにならない")
	}
	if i18n.CurrentLang() != i18n.LangJA {
		t.Error("保存失敗なのに言語を切り替えた")
	}
}

func TestService_OpenUpdate(t *testing.T) {
	const release = "https://github.com/ousiassllc/moleport/releases/tag/v9.9.9"
	tests := []struct {
		name     string
		enabled  bool
		ver      *servicetest.Version
		wantOpen bool
	}{
		{"新版あり", true, &servicetest.Version{OK: true, Result: core.VersionCheckResult{UpdateAvailable: true, LatestVersion: "v9.9.9", ReleaseURL: release}}, true},
		{"update_check 無効", false, &servicetest.Version{OK: true, Result: core.VersionCheckResult{UpdateAvailable: true, ReleaseURL: release}}, false},
		{"新版なし", true, &servicetest.Version{OK: true, Result: core.VersionCheckResult{ReleaseURL: release}}, false},
		{"未チェック", true, &servicetest.Version{}, false},
		{"想定外の URL", true, &servicetest.Version{OK: true, Result: core.VersionCheckResult{UpdateAvailable: true, ReleaseURL: "https://evil.example/releases/"}}, false},
		{"前方一致の偽装", true, &servicetest.Version{OK: true, Result: core.VersionCheckResult{UpdateAvailable: true, ReleaseURL: "https://github.com/ousiassllc/moleport.evil/releases/"}}, false},
		{"VersionChecker なし", true, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.UpdateCheck.Enabled = tt.enabled
			e := newEnv(cfg, tt.ver)
			err := e.s.OpenUpdate()
			if tt.wantOpen {
				if err != nil || !reflect.DeepEqual(e.open.urls, []string{release}) {
					t.Errorf("err=%v urls=%v", err, e.open.urls)
				}
				return
			}
			if !errors.Is(err, ErrNoUpdate) || len(e.open.urls) != 0 {
				t.Errorf("err=%v urls=%v, want ErrNoUpdate で開かない", err, e.open.urls)
			}
		})
	}
}

func TestService_NotReady(t *testing.T) {
	s := New(staticBackend{}, &fakeOpener{})
	if _, err := s.View(); !errors.Is(err, service.ErrNotReady) {
		t.Errorf("View = %v", err)
	}
	if _, err := s.Save(service.SettingsView{}); !errors.Is(err, service.ErrNotReady) {
		t.Errorf("Save = %v", err)
	}
	if err := s.OpenUpdate(); !errors.Is(err, service.ErrNotReady) {
		t.Errorf("OpenUpdate = %v", err)
	}
}
