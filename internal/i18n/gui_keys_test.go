package i18n

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// GUI のソースが i18n.T で参照するリテラルキー（"gui.…"）が ja / en の両方に存在することを検査する。
// 動的に組み立てるキー（"gui.status.host." + state 等）は末尾が "." のため対象外
// （それらは各 ViewModel のテストで検証する）。
var guiKeyPattern = regexp.MustCompile(`i18n\.T\("(gui\.[A-Za-z0-9_.]*[A-Za-z0-9_])"`)

func TestGUISourceKeysExistInAllLocales(t *testing.T) {
	root := os.DirFS(filepath.Join("..", "gui"))
	keys := map[string]string{} // key → 最初に見つかったファイル
	err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		for _, m := range guiKeyPattern.FindAllStringSubmatch(string(src), -1) {
			if _, ok := keys[m[1]]; !ok {
				keys[m[1]] = path
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/gui: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("GUI ソースから i18n キーが 1 件も見つからない（パターンまたはパスの誤り）")
	}

	resetLang(t)
	for _, lang := range SupportedLangs() {
		if err := SetLang(lang.Code); err != nil {
			t.Fatalf("SetLang(%q): %v", lang.Code, err)
		}
		for key, file := range keys {
			if got := T(key); got == key {
				t.Errorf("lang=%s: キー %q（%s）の翻訳がない", lang.Code, key, file)
			}
		}
	}
}
