package core

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

type failingStore struct{ YAMLStore }

func (failingStore) Write(string, interface{}) error { return errors.New("disk full") }

func TestConfigManager_UpdateConfig_ConcurrentHostsAndGet(t *testing.T) {
	cm := NewConfigManager(newTestStore(), t.TempDir())
	if _, err := cm.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			enabled := i%2 == 0
			_ = cm.UpdateConfig(func(cfg *Config) {
				if cfg.Hosts == nil {
					cfg.Hosts = map[string]HostConfig{}
				}
				cfg.Hosts["h"] = HostConfig{Reconnect: &ReconnectOverride{Enabled: &enabled}}
				delete(cfg.Hosts, "gone")
			})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			for _, hc := range cm.GetConfig().Hosts {
				_ = hc.Reconnect
			}
		}
	}()
	wg.Wait()
}

func TestConfigManager_UpdateConfig_WriteFailureKeepsCache(t *testing.T) {
	enabled := true
	base := NewConfigManager(newTestStore(), t.TempDir())
	if err := base.UpdateConfig(func(cfg *Config) {
		cfg.Hosts = map[string]HostConfig{"a": {Reconnect: &ReconnectOverride{Enabled: &enabled}}}
	}); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}
	cm := base.(*configManager)
	cm.store = failingStore{cm.store}
	before := cm.GetConfig()

	err := cm.UpdateConfig(func(cfg *Config) {
		*cfg.Hosts["a"].Reconnect.Enabled = false
		delete(cfg.Hosts, "a")
		cfg.Hosts["b"] = HostConfig{}
	})
	if err == nil {
		t.Fatal("UpdateConfig() error = nil, want write error")
	}
	if got := cm.GetConfig(); !reflect.DeepEqual(got, before) {
		t.Errorf("GetConfig() after failed update = %+v, want %+v", got, before)
	}
}

func TestConfigManager_ReturnedConfigIsIsolated(t *testing.T) {
	enabled := true
	cm := NewConfigManager(newTestStore(), t.TempDir())
	if err := cm.SaveConfig(&Config{Hosts: map[string]HostConfig{"a": {Reconnect: &ReconnectOverride{Enabled: &enabled}}}}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	got := cm.GetConfig()
	got.Hosts["z"] = HostConfig{}
	*got.Hosts["a"].Reconnect.Enabled = false

	again := cm.GetConfig()
	if _, ok := again.Hosts["z"]; ok || !*again.Hosts["a"].Reconnect.Enabled {
		t.Errorf("mutating a returned Config leaked into the cache: %+v", again.Hosts)
	}
}
