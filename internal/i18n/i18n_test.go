package i18n

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func resetLang(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { _ = SetLang(DefaultLang()) })
}

func TestSetLang_Valid(t *testing.T) {
	resetLang(t)

	tests := []struct {
		lang Lang
	}{
		{LangJA},
		{LangEN},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			if err := SetLang(tt.lang); err != nil {
				t.Fatalf("SetLang(%q) returned error: %v", tt.lang, err)
			}
			if got := CurrentLang(); got != tt.lang {
				t.Errorf("CurrentLang() = %q, want %q", got, tt.lang)
			}
		})
	}
}

func TestSetLang_Invalid(t *testing.T) {
	resetLang(t)

	err := SetLang(Lang("zz"))
	if err == nil {
		t.Fatal("SetLang(\"zz\") should return error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("error message should contain 'unsupported language', got: %v", err)
	}
}

func TestT_SimpleKey(t *testing.T) {
	resetLang(t)

	_ = SetLang(LangEN)
	got := T("cli.help.text")
	if got == "" {
		t.Error("T(\"cli.help.text\") returned empty string")
	}
	if got == "cli.help.text" {
		t.Error("T(\"cli.help.text\") returned the key itself (not found)")
	}
}

func TestT_WithTemplate(t *testing.T) {
	resetLang(t)

	_ = SetLang(LangEN)
	got := T("cli.daemon.started", map[string]any{"PID": 1234})
	if !strings.Contains(got, "1234") {
		t.Errorf("T with PID=1234 should contain '1234', got: %q", got)
	}
}

func TestT_MissingKey(t *testing.T) {
	resetLang(t)

	got := T("nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("T(\"nonexistent.key\") = %q, want %q", got, "nonexistent.key")
	}
}

func TestT_LanguageSwitch(t *testing.T) {
	resetLang(t)

	_ = SetLang(LangEN)
	en := T("cli.daemon.stopped")

	_ = SetLang(LangJA)
	ja := T("cli.daemon.stopped")

	if en == ja {
		t.Errorf("same key should return different text for en and ja, both returned: %q", en)
	}
	if en != "Daemon stopped" {
		t.Errorf("English text unexpected: %q", en)
	}
	if ja != "デーモンを停止しました" {
		t.Errorf("Japanese text unexpected: %q", ja)
	}
}

func TestSupportedLangs(t *testing.T) {
	langs := SupportedLangs()
	if len(langs) < 2 {
		t.Fatalf("SupportedLangs() returned %d items, want at least 2", len(langs))
	}

	codes := make(map[Lang]bool)
	for _, l := range langs {
		codes[l.Code] = true
	}
	for _, want := range []Lang{LangEN, LangJA} {
		if !codes[want] {
			t.Errorf("SupportedLangs() missing %q", want)
		}
	}
}

func TestSupportedLangs_ReturnsCopy(t *testing.T) {
	langs := SupportedLangs()
	langs[0].Label = "modified"

	original := SupportedLangs()
	if original[0].Label == "modified" {
		t.Error("SupportedLangs() should return a copy, not a reference")
	}
}

func TestDefaultLang(t *testing.T) {
	if got := DefaultLang(); got != LangEN {
		t.Errorf("DefaultLang() = %q, want %q", got, LangEN)
	}
}

func TestFlattenYAML(t *testing.T) {
	input := map[string]any{
		"a": map[string]any{
			"b": "value_ab",
			"c": map[string]any{
				"d": "value_acd",
			},
		},
		"e": "value_e",
		"f": 42,
	}

	result := make(map[string]string)
	flattenYAML(input, "", result)

	tests := []struct {
		key  string
		want string
	}{
		{"a.b", "value_ab"},
		{"a.c.d", "value_acd"},
		{"e", "value_e"},
		{"f", "42"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := result[tt.key]
			if !ok {
				t.Fatalf("key %q not found in result", tt.key)
			}
			if got != tt.want {
				t.Errorf("result[%q] = %q, want %q", tt.key, got, tt.want)
			}
		})
	}

	if len(result) != 4 {
		t.Errorf("flattenYAML produced %d entries, want 4", len(result))
	}
}

func TestFlattenYAML_WithPrefix(t *testing.T) {
	input := map[string]any{
		"key": "value",
	}

	result := make(map[string]string)
	flattenYAML(input, "prefix", result)

	got, ok := result["prefix.key"]
	if !ok {
		t.Fatal("key \"prefix.key\" not found")
	}
	if got != "value" {
		t.Errorf("result[\"prefix.key\"] = %q, want %q", got, "value")
	}
}

func TestLocaleKeyConsistency(t *testing.T) {
	// ja.yaml と en.yaml のキーが一致することを確認する。
	// 翻訳漏れがあればこのテストが失敗する。
	loadKeys := func(lang Lang) map[string]string {
		t.Helper()
		data, err := localeFS.ReadFile("locales/" + string(lang) + ".yaml")
		if err != nil {
			t.Fatalf("failed to read %s.yaml: %v", lang, err)
		}
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			t.Fatalf("failed to parse %s.yaml: %v", lang, err)
		}
		m := make(map[string]string)
		flattenYAML(raw, "", m)
		return m
	}

	enKeys := loadKeys(LangEN)
	jaKeys := loadKeys(LangJA)

	for k := range enKeys {
		if _, ok := jaKeys[k]; !ok {
			t.Errorf("key %q exists in en.yaml but missing in ja.yaml", k)
		}
	}
	for k := range jaKeys {
		if _, ok := enKeys[k]; !ok {
			t.Errorf("key %q exists in ja.yaml but missing in en.yaml", k)
		}
	}
}

func TestT_NoTemplateData(t *testing.T) {
	resetLang(t)

	_ = SetLang(LangEN)
	got := T("cli.daemon.stopped")
	if got != "Daemon stopped" {
		t.Errorf("T without data = %q, want %q", got, "Daemon stopped")
	}
}
