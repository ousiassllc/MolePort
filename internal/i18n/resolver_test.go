package i18n

import (
	"testing"
)

func TestResolve_ConfigLang(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "")

	got := Resolve("ja")
	if got != LangJA {
		t.Errorf("Resolve(\"ja\") = %q, want %q", got, LangJA)
	}
}

func TestResolve_ConfigLangPriority(t *testing.T) {
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("LANG", "en_US.UTF-8")

	got := Resolve("ja")
	if got != LangJA {
		t.Errorf("Resolve(\"ja\") with en env = %q, want %q (config should have priority)", got, LangJA)
	}
}

func TestResolve_EnvLCAll(t *testing.T) {
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	t.Setenv("LANG", "")

	got := Resolve("")
	if got != LangJA {
		t.Errorf("Resolve(\"\") with LC_ALL=ja_JP.UTF-8 = %q, want %q", got, LangJA)
	}
}

func TestResolve_EnvLang(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "ja_JP.UTF-8")

	got := Resolve("")
	if got != LangJA {
		t.Errorf("Resolve(\"\") with LANG=ja_JP.UTF-8 = %q, want %q", got, LangJA)
	}
}

func TestResolve_EnvPriority(t *testing.T) {
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	t.Setenv("LANG", "en_US.UTF-8")

	got := Resolve("")
	if got != LangJA {
		t.Errorf("Resolve(\"\") LC_ALL=ja, LANG=en = %q, want %q (LC_ALL should override)", got, LangJA)
	}
}

func TestResolve_Default(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "")

	got := Resolve("")
	if got != LangEN {
		t.Errorf("Resolve(\"\") with no env = %q, want %q", got, LangEN)
	}
}

func TestResolve_UnsupportedConfigLang(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "")

	got := Resolve("zz")
	if got != LangEN {
		t.Errorf("Resolve(\"zz\") = %q, want %q (should fallback to default)", got, LangEN)
	}
}

func TestResolve_UnsupportedEnvLang(t *testing.T) {
	t.Setenv("LC_ALL", "zz_ZZ.UTF-8")
	t.Setenv("LANG", "")

	got := Resolve("")
	if got != LangEN {
		t.Errorf("Resolve(\"\") with LC_ALL=zz = %q, want %q (should fallback)", got, LangEN)
	}
}

func TestParseLangEnv(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ja_JP.UTF-8", "ja"},
		{"en_US", "en"},
		{"ja", "ja"},
		{"C", ""},
		{"POSIX", ""},
		{"", ""},
		{"en_GB.UTF-8", "en"},
		{"fr_FR.ISO-8859-1", "fr"},
		{"zh_CN", "zh"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLangEnv(tt.input)
			if got != tt.want {
				t.Errorf("ParseLangEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
