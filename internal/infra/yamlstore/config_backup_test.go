package yamlstore

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

const brokenConfigYAML = "forwards:\n  - name: keep-me\n   host: [unclosed\n"

func newBrokenConfigManager(t *testing.T) (core.ConfigManager, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(brokenConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cm := core.NewConfigManager(NewYAMLStore(), dir)
	if _, err := cm.LoadConfig(); err == nil {
		t.Fatal("LoadConfig() should fail for broken YAML")
	}
	return cm, path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // テスト用の一時ディレクトリ内のパス
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// 読み込みに失敗した config.yaml は、UpdateConfig の書き込み前に config.yaml.bak へ退避される。
func TestConfigManager_UpdateConfigBacksUpUnreadableConfig(t *testing.T) {
	cm, path := newBrokenConfigManager(t)

	if err := cm.UpdateConfig(func(c *core.Config) { c.Language = "ja" }); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	if got := readFile(t, path+".bak"); got != brokenConfigYAML {
		t.Errorf("backup = %q, want original %q", got, brokenConfigYAML)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path + ".bak")
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("backup perm = %o, want no group/other access", perm)
		}
	}

	reloaded, err := core.NewConfigManager(NewYAMLStore(), filepath.Dir(path)).LoadConfig()
	if err != nil {
		t.Fatalf("reload after UpdateConfig error = %v", err)
	}
	if reloaded.Language != "ja" {
		t.Errorf("Language = %q, want ja", reloaded.Language)
	}
}

// 退避は最初の書き込みだけで行い、その後の書き込みで .bak を上書きしない。
func TestConfigManager_BacksUpOnlyOnce(t *testing.T) {
	cm, path := newBrokenConfigManager(t)

	for _, lang := range []string{"ja", "en"} {
		if err := cm.UpdateConfig(func(c *core.Config) { c.Language = lang }); err != nil {
			t.Fatalf("UpdateConfig(%s) error = %v", lang, err)
		}
	}

	if got := readFile(t, path+".bak"); got != brokenConfigYAML {
		t.Errorf("backup = %q, want original %q", got, brokenConfigYAML)
	}
}

// 退避できない場合は元の config.yaml を保護するため書き込みを拒否する。
func TestConfigManager_RefusesToOverwriteWhenBackupFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(brokenConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	// .bak をディレクトリにして退避の書き込みを失敗させる。
	if err := os.Mkdir(path+".bak", 0o700); err != nil {
		t.Fatal(err)
	}
	cm := core.NewConfigManager(NewYAMLStore(), dir)
	if _, err := cm.LoadConfig(); err == nil {
		t.Fatal("LoadConfig() should fail for broken YAML")
	}

	if err := cm.UpdateConfig(func(c *core.Config) { c.Language = "ja" }); err == nil {
		t.Fatal("UpdateConfig() should refuse to overwrite when backup fails")
	}
	if got := readFile(t, path); got != brokenConfigYAML {
		t.Errorf("config.yaml was modified: %q", got)
	}
}

// 読み込みに成功している場合、および未作成のファイルでは退避ファイルを作らない。
func TestConfigManager_NoBackupWhenLoadSucceeds(t *testing.T) {
	dir := t.TempDir()
	cm := core.NewConfigManager(NewYAMLStore(), dir)
	if _, err := cm.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	if err := cm.UpdateConfig(func(c *core.Config) { c.Language = "ja" }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml.bak")); !os.IsNotExist(err) {
		t.Errorf("backup should not exist, stat err = %v", err)
	}
}
