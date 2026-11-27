package core

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
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

// gatedForwardManager は最初の GetRules でスナップショットを取った直後に、
// 並行する別の保存が完了するのを（上限付きで）待つ ForwardManager のテスト用スタブ。
type gatedForwardManager struct {
	ForwardManager
	mu          sync.Mutex
	rules       []ForwardRule
	calls       int
	snapshotted chan struct{}
	release     chan struct{}
}

func (m *gatedForwardManager) GetRules() []ForwardRule {
	m.mu.Lock()
	snapshot := append([]ForwardRule(nil), m.rules...)
	m.calls++
	first := m.calls == 1
	m.mu.Unlock()

	if first {
		close(m.snapshotted)
		// スナップショットをロック外で取る実装では、この間に別の保存が完了してしまう。
		// ロック内で取る実装では別の保存はロック待ちになるため、上限で待ちを打ち切る。
		select {
		case <-m.release:
		case <-time.After(50 * time.Millisecond):
		}
	}
	return snapshot
}

func (m *gatedForwardManager) add(rule ForwardRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// 古いスナップショットを持つ保存が、新しいスナップショットの保存を上書きしない。
func TestPersistForwardRules_ConcurrentSnapshotsDoNotLoseRules(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManager(newTestStore(), dir)
	r1 := ForwardRule{Name: "r1", Host: "dev", Type: Local, LocalPort: 1, RemoteHost: "h", RemotePort: 2}
	r2 := ForwardRule{Name: "r2", Host: "dev", Type: Local, LocalPort: 3, RemoteHost: "h", RemotePort: 4}
	fwd := &gatedForwardManager{
		rules:       []ForwardRule{r1},
		snapshotted: make(chan struct{}),
		release:     make(chan struct{}),
	}

	errs := make(chan error, 2)
	go func() { errs <- PersistForwardRules(cm, fwd) }()

	<-fwd.snapshotted
	fwd.add(r2)
	go func() {
		err := PersistForwardRules(cm, fwd)
		close(fwd.release)
		errs <- err
	}()

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("PersistForwardRules() error = %v", err)
		}
	}

	reloaded, err := NewConfigManager(newTestStore(), dir).LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := []ForwardRule{r1, r2}
	if !reflect.DeepEqual(reloaded.Forwards, want) {
		t.Errorf("Forwards = %+v, want %+v", reloaded.Forwards, want)
	}
}
