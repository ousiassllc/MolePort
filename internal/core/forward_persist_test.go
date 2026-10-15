package core

import (
	"errors"
	"reflect"
	"testing"
)

// rulesOnlyForwardManager は GetRules だけを実装する ForwardManager のテスト用スタブ。
type rulesOnlyForwardManager struct {
	ForwardManager
	rules []ForwardRule
}

func (m *rulesOnlyForwardManager) GetRules() []ForwardRule { return m.rules }

// failingConfigManager は UpdateConfig が常に失敗する ConfigManager のテスト用スタブ。
type failingConfigManager struct {
	ConfigManager
	err error
}

func (m *failingConfigManager) UpdateConfig(func(*Config)) error { return m.err }

func TestPersistForwardRules_SavesRulesToConfig(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManager(newTestStore(), dir)
	rules := []ForwardRule{
		{Name: "web", Host: "dev", Type: Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
		{Name: "db", Host: "dev", Type: Local, LocalPort: 5433, RemoteHost: "localhost", RemotePort: 5432},
	}

	if err := PersistForwardRules(cm, &rulesOnlyForwardManager{rules: rules}); err != nil {
		t.Fatalf("PersistForwardRules() error = %v", err)
	}

	// 別の ConfigManager で読み直し、ファイルに保存されていることを確認する。
	reloaded, err := NewConfigManager(newTestStore(), dir).LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !reflect.DeepEqual(reloaded.Forwards, rules) {
		t.Errorf("Forwards = %+v, want %+v", reloaded.Forwards, rules)
	}
}

func TestPersistForwardRules_ReplacesExistingRules(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManager(newTestStore(), dir)
	first := []ForwardRule{{Name: "old", Host: "dev", Type: Local, LocalPort: 1, RemoteHost: "h", RemotePort: 2}}
	if err := PersistForwardRules(cm, &rulesOnlyForwardManager{rules: first}); err != nil {
		t.Fatalf("first PersistForwardRules() error = %v", err)
	}

	if err := PersistForwardRules(cm, &rulesOnlyForwardManager{}); err != nil {
		t.Fatalf("second PersistForwardRules() error = %v", err)
	}

	if got := cm.GetConfig().Forwards; len(got) != 0 {
		t.Errorf("Forwards = %+v, want empty", got)
	}
}

func TestPersistForwardRules_UpdateConfigError(t *testing.T) {
	wantErr := errors.New("disk full")
	err := PersistForwardRules(&failingConfigManager{err: wantErr}, &rulesOnlyForwardManager{})
	if !errors.Is(err, wantErr) {
		t.Errorf("PersistForwardRules() error = %v, want wrapping %v", err, wantErr)
	}
}
