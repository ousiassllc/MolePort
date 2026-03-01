package i18n

import (
	"embed"
	"fmt"
	"strings"
	"sync"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed locales/*.yaml
var localeFS embed.FS

// Lang は対応言語を表す型。
type Lang string

const (
	LangJA Lang = "ja"
	LangEN Lang = "en"
)

// LangInfo は言語の表示情報を保持する。
type LangInfo struct {
	Code  Lang
	Label string // その言語自体での表示名
}

// supportedLangs は対応言語一覧。
var supportedLangs = []LangInfo{
	{Code: LangEN, Label: "English"},
	{Code: LangJA, Label: "日本語"},
}

// SupportedLangs は対応言語の一覧を返す。
func SupportedLangs() []LangInfo {
	result := make([]LangInfo, len(supportedLangs))
	copy(result, supportedLangs)
	return result
}

// DefaultLang はデフォルト言語を返す。
func DefaultLang() Lang { return LangEN }

// localizer はグローバルな翻訳管理インスタンス。
type localizer struct {
	mu        sync.RWMutex
	lang      Lang
	messages  map[string]string
	tmplCache map[string]*template.Template
}

var global = &localizer{
	lang:      DefaultLang(),
	messages:  make(map[string]string),
	tmplCache: make(map[string]*template.Template),
}

func init() {
	if err := SetLang(DefaultLang()); err != nil {
		panic("i18n: failed to load default locale: " + err.Error())
	}
}

// SetLang は現在の言語を設定し、翻訳データをロードする。
func SetLang(lang Lang) error {
	data, err := localeFS.ReadFile(fmt.Sprintf("locales/%s.yaml", string(lang)))
	if err != nil {
		return fmt.Errorf("i18n: unsupported language %q: %w", lang, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("i18n: failed to parse %s.yaml: %w", lang, err)
	}

	messages := make(map[string]string)
	flattenYAML(raw, "", messages)

	global.mu.Lock()
	defer global.mu.Unlock()
	global.lang = lang
	global.messages = messages
	global.tmplCache = make(map[string]*template.Template)
	return nil
}

// CurrentLang は現在設定されている言語を返す。
func CurrentLang() Lang {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.lang
}

// T は翻訳キーに対応するテキストを返す。
// data が指定された場合、text/template で変数を展開する。
// キーが見つからない場合はキー自体を返す。
//
// NOTE: RUnlock は defer ではなく手動で呼び出す。テンプレートのパースは
// 比較的重い処理であり、読み取りロックを保持したまま実行すると
// SetLang の呼び出しをブロックするため。
func T(key string, data ...any) string {
	global.mu.RLock()
	msg, ok := global.messages[key]
	if !ok {
		global.mu.RUnlock()
		return key
	}

	if len(data) == 0 || !strings.Contains(msg, "{{") {
		global.mu.RUnlock()
		return msg
	}

	tmpl, cached := global.tmplCache[key]
	global.mu.RUnlock()

	if !cached {
		var err error
		tmpl, err = template.New(key).Parse(msg)
		if err != nil {
			return msg
		}
		global.mu.Lock()
		global.tmplCache[key] = tmpl
		global.mu.Unlock()
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data[0]); err != nil {
		return msg
	}
	return buf.String()
}

// flattenYAML は YAML のネスト構造をドット区切りフラットマップに変換する。
func flattenYAML(data map[string]any, prefix string, result map[string]string) {
	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flattenYAML(val, key, result)
		case string:
			result[key] = val
		default:
			result[key] = fmt.Sprintf("%v", val)
		}
	}
}
