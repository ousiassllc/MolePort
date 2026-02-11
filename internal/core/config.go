package core

import (
	"os"
	"path/filepath"
	"sync"
)

// YAMLStore は YAML ファイルの読み書きを担う。
// infra.YAMLStore と同じインターフェースで、import cycle を回避するために core で定義する。
type YAMLStore interface {
	Read(path string, dest interface{}) error
	Write(path string, data interface{}) error
	Exists(path string) bool
}

// ConfigManager はアプリケーション設定と状態の管理を担う。
type ConfigManager interface {
	LoadConfig() (*Config, error)
	SaveConfig(config *Config) error
	GetConfig() *Config
	UpdateConfig(fn func(*Config)) error
	LoadState() (*State, error)
	SaveState(state *State) error
	DeleteState() error
	ConfigDir() string
}

type configManager struct {
	mu        sync.RWMutex
	store     YAMLStore
	configDir string
	cached    *Config
}

// NewConfigManager は ConfigManager の実装を返す。
func NewConfigManager(store YAMLStore, configDir string) ConfigManager {
	return &configManager{
		store:     store,
		configDir: configDir,
	}
}

func (m *configManager) configPath() string {
	return filepath.Join(m.configDir, "config.yaml")
}

func (m *configManager) statePath() string {
	return filepath.Join(m.configDir, "state.yaml")
}

// LoadConfig は config.yaml を読み込み、キャッシュに保存する。
// ファイルが存在しない場合はデフォルト設定を返す。
func (m *configManager) LoadConfig() (*Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := DefaultConfig()
	if err := m.store.Read(m.configPath(), &cfg); err != nil {
		return nil, err
	}
	m.cached = &cfg
	return &cfg, nil
}

// SaveConfig は設定を config.yaml に書き込み、キャッシュを更新する。
func (m *configManager) SaveConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.store.Write(m.configPath(), config); err != nil {
		return err
	}
	c := *config
	m.cached = &c
	return nil
}

// GetConfig はキャッシュされた設定を返す。
// LoadConfig が呼ばれていない場合はデフォルト設定を返す。
func (m *configManager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.cached == nil {
		cfg := DefaultConfig()
		return &cfg
	}
	c := *m.cached
	return &c
}

// UpdateConfig は設定をアトミックに変更して保存する。
func (m *configManager) UpdateConfig(fn func(*Config)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cfg Config
	if m.cached != nil {
		cfg = *m.cached
	} else {
		cfg = DefaultConfig()
	}

	fn(&cfg)

	if err := m.store.Write(m.configPath(), &cfg); err != nil {
		return err
	}
	m.cached = &cfg
	return nil
}

// LoadState は state.yaml を読み込む。
func (m *configManager) LoadState() (*State, error) {
	var state State
	if err := m.store.Read(m.statePath(), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveState は状態を state.yaml に書き込む。
func (m *configManager) SaveState(state *State) error {
	return m.store.Write(m.statePath(), state)
}

// DeleteState は state.yaml を削除する。
// ファイルが存在しない場合はエラーを返さない。
func (m *configManager) DeleteState() error {
	err := os.Remove(m.statePath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ConfigDir は設定ディレクトリのパスを返す。
func (m *configManager) ConfigDir() string {
	return m.configDir
}
