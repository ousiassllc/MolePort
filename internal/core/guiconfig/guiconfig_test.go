package guiconfig

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefault(t *testing.T) {
	got := Default()
	if got.Theme != (Theme{Base: "system", Accent: "violet"}) || got.TrayNoticeShown {
		t.Errorf("Default() = %+v", got)
	}
}

func TestValidBaseAndAccent(t *testing.T) {
	for _, b := range Bases() {
		if !ValidBase(b) {
			t.Errorf("ValidBase(%q) = false", b)
		}
	}
	for _, a := range Accents() {
		if !ValidAccent(a) {
			t.Errorf("ValidAccent(%q) = false", a)
		}
	}
	for _, bad := range []string{"", "sepia", "Dark"} {
		if ValidBase(bad) || ValidAccent(bad) {
			t.Errorf("%q が有効と判定された", bad)
		}
	}
	if len(Bases()) != 3 || len(Accents()) != 5 {
		t.Errorf("選択肢の数: base=%d accent=%d, want 3 / 5", len(Bases()), len(Accents()))
	}
}

func TestBasesReturnsCopy(t *testing.T) {
	Bases()[0] = "mutated"
	if !ValidBase("system") {
		t.Error("Bases() の戻り値の変更が内部の選択肢に影響した")
	}
}

func TestTheme_Normalized(t *testing.T) {
	tests := []struct {
		in, want Theme
	}{
		{Theme{Base: "dark", Accent: "orange"}, Theme{Base: "dark", Accent: "orange"}},
		{Theme{}, Theme{Base: "system", Accent: "violet"}},
		{Theme{Base: "sepia", Accent: "cyan"}, Theme{Base: "system", Accent: "cyan"}},
	}
	for _, tt := range tests {
		if got := tt.in.Normalized(); got != tt.want {
			t.Errorf("%+v.Normalized() = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

// 既定値を入れた構造体に YAML を重ねると、未指定のキーは既定値のまま残る（core.LoadConfig と同じ読み方）。
func TestConfig_YAMLKeepsDefaultsForMissingKeys(t *testing.T) {
	cfg := Default()
	if err := yaml.Unmarshal([]byte("theme:\n  accent: cyan\ntray_notice_shown: true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	want := Config{Theme: Theme{Base: "system", Accent: "cyan"}, TrayNoticeShown: true}
	if cfg != want {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	const wantYAML = "theme:\n    base: system\n    accent: cyan\ntray_notice_shown: true\n"
	if string(out) != wantYAML {
		t.Errorf("Marshal =\n%s\nwant\n%s", out, wantYAML)
	}
}
