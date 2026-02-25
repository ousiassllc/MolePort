package core

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConfigManager_LoadState_Empty(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	state, err := cm.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if state.SelectedHost != "" {
		t.Errorf("SelectedHost = %q, want empty", state.SelectedHost)
	}
}

func TestConfigManager_SaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	now := time.Now().Truncate(time.Second)
	state := &State{
		LastUpdated: now,
		ActiveForwards: []ForwardRule{
			{Name: "web", Host: "server", Type: Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
		},
		SelectedHost: "server",
	}

	if err := cm.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	loaded, err := cm.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if loaded.SelectedHost != "server" {
		t.Errorf("SelectedHost = %q, want %q", loaded.SelectedHost, "server")
	}
	if len(loaded.ActiveForwards) != 1 {
		t.Fatalf("len(ActiveForwards) = %d, want 1", len(loaded.ActiveForwards))
	}
	if loaded.ActiveForwards[0].Name != "web" {
		t.Errorf("ActiveForwards[0].Name = %q, want %q", loaded.ActiveForwards[0].Name, "web")
	}
}

func TestConfigManager_DeleteState(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	// state.yaml を作成
	state := &State{
		LastUpdated: time.Now(),
		ActiveForwards: []ForwardRule{
			{Name: "web", Host: "server", Type: Local, LocalPort: 8080},
		},
	}
	if err := cm.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// state.yaml が存在することを確認
	if !store.Exists(filepath.Join(dir, "state.yaml")) {
		t.Fatal("state.yaml should exist after SaveState")
	}

	// 削除
	if err := cm.DeleteState(); err != nil {
		t.Fatalf("DeleteState() error = %v", err)
	}

	// state.yaml が消えたことを確認
	if store.Exists(filepath.Join(dir, "state.yaml")) {
		t.Error("state.yaml should not exist after DeleteState")
	}

	// 存在しないファイルの削除はエラーにならない
	if err := cm.DeleteState(); err != nil {
		t.Errorf("DeleteState() on non-existent file should not error, got %v", err)
	}
}

func TestConfigManager_ConfigDir(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	if cm.ConfigDir() != dir {
		t.Errorf("ConfigDir() = %q, want %q", cm.ConfigDir(), dir)
	}
}

func TestConfigManager_ConfigPath(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	// config.yaml がディレクトリ内に作成されることを確認
	cfg := &Config{SSHConfigPath: "/test"}
	if err := cm.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if !store.Exists(filepath.Join(dir, "config.yaml")) {
		t.Error("config.yaml should exist after SaveConfig")
	}
}

func TestConfigManager_Concurrent(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore()
	cm := NewConfigManager(store, dir)

	if _, err := cm.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cm.GetConfig()
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = cm.UpdateConfig(func(cfg *Config) {
				cfg.Reconnect.MaxRetries = i
			})
		}(i)
	}
	wg.Wait()
}
