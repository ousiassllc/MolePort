package yamlstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/guiconfig"
)

// 実際の YAMLStore で config.yaml の gui: セクションを読み書きできること、
// gui: のない既存ファイルが ConfigManager 経由で既定値付きで読めることを確認する。
func TestYAMLStore_ConfigGUISectionCompat(t *testing.T) {
	dir := t.TempDir()
	legacy := "language: en\nupdate_check:\n  enabled: false\n  interval: 24h\ntui:\n  theme:\n    base: dark\n    accent: blue\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	cm := core.NewConfigManager(NewYAMLStore(), dir)

	cfg, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.GUI != guiconfig.Default() || cfg.UpdateCheck.Enabled || cfg.TUI.Theme.Base != "dark" {
		t.Fatalf("既存ファイルの読み込み: %+v", cfg)
	}

	if err := cm.UpdateConfig(func(c *core.Config) {
		c.GUI.Theme = guiconfig.Theme{Base: "light", Accent: "orange"}
		c.GUI.TrayNoticeShown = true
	}); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}

	reloaded, err := core.NewConfigManager(NewYAMLStore(), dir).LoadConfig()
	if err != nil {
		t.Fatalf("再読み込み: %v", err)
	}
	want := guiconfig.Config{Theme: guiconfig.Theme{Base: "light", Accent: "orange"}, TrayNoticeShown: true}
	if reloaded.GUI != want {
		t.Errorf("GUI = %+v, want %+v", reloaded.GUI, want)
	}
	if reloaded.TUI.Theme != (core.ThemeConfig{Base: "dark", Accent: "blue"}) || reloaded.UpdateCheck.Enabled {
		t.Errorf("他のキーが変わった: %+v", reloaded)
	}
}
