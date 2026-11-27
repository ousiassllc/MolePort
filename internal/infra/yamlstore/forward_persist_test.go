package yamlstore

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// gatedForwardManager は最初の GetRules でスナップショットを取った直後に、
// 並行する別の保存が完了するのを（上限付きで）待つ ForwardManager のテスト用スタブ。
type gatedForwardManager struct {
	core.ForwardManager
	mu          sync.Mutex
	rules       []core.ForwardRule
	calls       int
	snapshotted chan struct{}
	release     chan struct{}
}

func (m *gatedForwardManager) GetRules() []core.ForwardRule {
	m.mu.Lock()
	snapshot := append([]core.ForwardRule(nil), m.rules...)
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

func (m *gatedForwardManager) add(rule core.ForwardRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// 古いスナップショットを持つ保存が、新しいスナップショットの保存を上書きしない。
func TestPersistForwardRules_ConcurrentSnapshotsDoNotLoseRules(t *testing.T) {
	dir := t.TempDir()
	cm := core.NewConfigManager(NewYAMLStore(), dir)
	r1 := core.ForwardRule{Name: "r1", Host: "dev", Type: core.Local, LocalPort: 1, RemoteHost: "h", RemotePort: 2}
	r2 := core.ForwardRule{Name: "r2", Host: "dev", Type: core.Local, LocalPort: 3, RemoteHost: "h", RemotePort: 4}
	fwd := &gatedForwardManager{
		rules:       []core.ForwardRule{r1},
		snapshotted: make(chan struct{}),
		release:     make(chan struct{}),
	}

	errs := make(chan error, 2)
	go func() { errs <- core.PersistForwardRules(cm, fwd) }()

	<-fwd.snapshotted
	fwd.add(r2)
	go func() {
		err := core.PersistForwardRules(cm, fwd)
		close(fwd.release)
		errs <- err
	}()

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("core.PersistForwardRules() error = %v", err)
		}
	}

	reloaded, err := core.NewConfigManager(NewYAMLStore(), dir).LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := []core.ForwardRule{r1, r2}
	if !reflect.DeepEqual(reloaded.Forwards, want) {
		t.Errorf("Forwards = %+v, want %+v", reloaded.Forwards, want)
	}
}
