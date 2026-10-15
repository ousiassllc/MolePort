package view_test

import (
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/molecules"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
)

func TestThemeSwatch(t *testing.T) {
	out := renderToString(t, molecules.ThemeSwatch(molecules.ThemeSwatchProps{Name: "theme_accent", Value: "blue", Label: "Blue", Checked: true}))
	for _, s := range []string{`type="radio"`, `name="theme_accent"`, `value="blue"`, "checked", "data-theme-preview", `data-swatch="theme_accent:blue"`, "Blue"} {
		if !strings.Contains(out, s) {
			t.Errorf("%q が無い: %s", s, out)
		}
	}
	if out := renderToString(t, molecules.ThemeSwatch(molecules.ThemeSwatchProps{Name: "theme_base", Value: "dark"})); strings.Contains(out, "checked") {
		t.Errorf("未選択なのに checked: %s", out)
	}
}

func TestSettingsForm_ShowsErrorsAndRefetchesOnSettingsTopic(t *testing.T) {
	out := renderToString(t, organisms.SettingsForm(
		service.SettingsView{Language: "ja", ThemeBase: "system", ThemeAccent: "green"},
		service.FormErrors{"theme_base": "テーマが不正", "_form": "保存に失敗"},
	))
	for _, s := range []string{
		`hx-trigger="mp:settings from:body"`, `hx-swap="none"`,
		`<option value="ja" selected`, `value="system" checked`, `value="green" checked`,
		"テーマが不正", "保存に失敗",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("%q が無い: %s", s, out)
		}
	}
	if strings.Contains(out, `name="update_check_enabled" checked`) {
		t.Error("update_check 無効なのに checked")
	}
	if strings.Count(out, `name="theme_base"`) != 3 || strings.Count(out, `name="theme_accent"`) != 5 {
		t.Errorf("スウォッチの数が不正: %s", out)
	}
}
