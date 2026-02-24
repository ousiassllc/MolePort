package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// testYAMLStore は core.YAMLStore のテスト用実装。infra.yamlStore と同等の機能を持つ。
type testYAMLStore struct{}

func (s *testYAMLStore) Read(path string, dest interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, dest)
}

func (s *testYAMLStore) Write(path string, data interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0600)
}

func (s *testYAMLStore) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func newTestStore() *testYAMLStore {
	return &testYAMLStore{}
}

func TestConfigManager_LoadConfig_Default(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	cfg, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	want := DefaultConfig()
	if cfg.SSHConfigPath != want.SSHConfigPath {
		t.Errorf("SSHConfigPath = %q, want %q", cfg.SSHConfigPath, want.SSHConfigPath)
	}
	if cfg.Reconnect.MaxRetries != want.Reconnect.MaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.Reconnect.MaxRetries, want.Reconnect.MaxRetries)
	}
	if cfg.Log.Level != want.Log.Level {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, want.Log.Level)
	}
}

func TestConfigManager_SaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	cfg := &Config{
		SSHConfigPath: "/custom/ssh/config",
		Reconnect: ReconnectConfig{
			Enabled:      true,
			MaxRetries:   5,
			InitialDelay: Duration{Duration: 2 * time.Second},
			MaxDelay:     Duration{Duration: 30 * time.Second},
		},
		Session: SessionConfig{AutoRestore: false},
		Log:     LogConfig{Level: "debug", File: "/tmp/test.log"},
		Forwards: []ForwardRule{
			{Name: "test", Host: "server", Type: Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
		},
	}

	if err := cm.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// 新しい ConfigManager で読み込む
	cm2 := NewConfigManager(store, dir)
	loaded, err := cm2.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.SSHConfigPath != cfg.SSHConfigPath {
		t.Errorf("SSHConfigPath = %q, want %q", loaded.SSHConfigPath, cfg.SSHConfigPath)
	}
	if loaded.Reconnect.MaxRetries != cfg.Reconnect.MaxRetries {
		t.Errorf("MaxRetries = %d, want %d", loaded.Reconnect.MaxRetries, cfg.Reconnect.MaxRetries)
	}
	if loaded.Session.AutoRestore != cfg.Session.AutoRestore {
		t.Errorf("AutoRestore = %v, want %v", loaded.Session.AutoRestore, cfg.Session.AutoRestore)
	}
	if len(loaded.Forwards) != 1 {
		t.Fatalf("len(Forwards) = %d, want 1", len(loaded.Forwards))
	}
	if loaded.Forwards[0].Name != "test" {
		t.Errorf("Forwards[0].Name = %q, want %q", loaded.Forwards[0].Name, "test")
	}
}

func TestConfigManager_GetConfig_BeforeLoad(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	cfg := cm.GetConfig()
	want := DefaultConfig()
	if cfg.SSHConfigPath != want.SSHConfigPath {
		t.Errorf("SSHConfigPath = %q, want %q", cfg.SSHConfigPath, want.SSHConfigPath)
	}
}

func TestConfigManager_GetConfig_AfterLoad(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	saved := &Config{
		SSHConfigPath: "/custom/path",
		Reconnect:     ReconnectConfig{MaxRetries: 3},
		Log:           LogConfig{Level: "debug"},
	}
	if err := cm.SaveConfig(saved); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	got := cm.GetConfig()
	if got.SSHConfigPath != "/custom/path" {
		t.Errorf("SSHConfigPath = %q, want %q", got.SSHConfigPath, "/custom/path")
	}
}

func TestConfigManager_UpdateConfig(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	// 先にロード
	if _, err := cm.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	err := cm.UpdateConfig(func(cfg *Config) {
		cfg.SSHConfigPath = "/updated/path"
		cfg.Reconnect.MaxRetries = 20
	})
	if err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	got := cm.GetConfig()
	if got.SSHConfigPath != "/updated/path" {
		t.Errorf("SSHConfigPath = %q, want %q", got.SSHConfigPath, "/updated/path")
	}
	if got.Reconnect.MaxRetries != 20 {
		t.Errorf("MaxRetries = %d, want 20", got.Reconnect.MaxRetries)
	}

	// ファイルにも永続化されていることを確認
	cm2 := NewConfigManager(store, dir)
	loaded, err := cm2.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if loaded.SSHConfigPath != "/updated/path" {
		t.Errorf("persisted SSHConfigPath = %q, want %q", loaded.SSHConfigPath, "/updated/path")
	}
}

func TestConfigManager_UpdateConfig_BeforeLoad(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	err := cm.UpdateConfig(func(cfg *Config) {
		cfg.SSHConfigPath = "/new/path"
	})
	if err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	got := cm.GetConfig()
	if got.SSHConfigPath != "/new/path" {
		t.Errorf("SSHConfigPath = %q, want %q", got.SSHConfigPath, "/new/path")
	}
	// デフォルト値が保持されていることを確認
	if got.Reconnect.MaxRetries != 10 {
		t.Errorf("MaxRetries = %d, want 10 (default)", got.Reconnect.MaxRetries)
	}
}

func TestConfigManager_LoadConfig_MergesDefaults(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()

	// 部分的な設定を直接書き込む
	partial := map[string]interface{}{
		"ssh_config_path": "/custom/path",
	}
	if err := store.Write(filepath.Join(dir, "config.yaml"), partial); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	cm := NewConfigManager(store, dir)
	cfg, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// カスタム値が読み込まれていること
	if cfg.SSHConfigPath != "/custom/path" {
		t.Errorf("SSHConfigPath = %q, want %q", cfg.SSHConfigPath, "/custom/path")
	}

	// デフォルト値がマージされていること
	if !cfg.Reconnect.Enabled {
		t.Error("Reconnect.Enabled should be true (default)")
	}
	if cfg.Reconnect.MaxRetries != 10 {
		t.Errorf("MaxRetries = %d, want 10 (default)", cfg.Reconnect.MaxRetries)
	}
}
