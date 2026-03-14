package i18n

import (
	"os"
	"strings"
)

// Resolve は使用する言語を解決する。
// 優先順位: configLang -> LC_ALL -> LANG -> DefaultLang。
func Resolve(configLang string) Lang {
	if configLang != "" {
		lang := Lang(configLang)
		if isSupported(lang) {
			return lang
		}
	}

	for _, envKey := range []string{"LC_ALL", "LANG"} {
		if envVal := os.Getenv(envKey); envVal != "" {
			if code := ParseLangEnv(envVal); code != "" {
				lang := Lang(code)
				if isSupported(lang) {
					return lang
				}
			}
		}
	}

	return DefaultLang()
}

// ParseLangEnv は環境変数の値から言語コードを抽出する。
// 例: "ja_JP.UTF-8" -> "ja", "en_US" -> "en", "C" -> ""
func ParseLangEnv(envValue string) string {
	if envValue == "" || envValue == "C" || envValue == "POSIX" {
		return ""
	}
	code := strings.SplitN(envValue, ".", 2)[0]
	code = strings.SplitN(code, "_", 2)[0]
	return code
}

// isSupported は指定された言語が対応言語かを返す。
func isSupported(lang Lang) bool {
	for _, l := range supportedLangs {
		if l.Code == lang {
			return true
		}
	}
	return false
}
