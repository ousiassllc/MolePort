package daemon

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// 復元が接続待ちの途中で Stop しても、処理中・未着手のルールが state.yaml から消えない（#159 / #154）。
func TestDaemon_Stop_KeepsUnrestoredRulesInState(t *testing.T) {
	rules := []core.ForwardRule{{Name: "done"}, {Name: "dialing"}, {Name: "waiting"}}
	entered := make(chan struct{})
	var saved *core.State

	fwd := &lifecycleFwdMgr{}
	cfgMgr := &mockConfigManagerForState{
		config:      &core.Config{Session: core.SessionConfig{AutoRestore: true}},
		loadStateFn: func() (*core.State, error) { return &core.State{ActiveForwards: rules}, nil },
		saveStateFn: func(s *core.State) error { saved = s; return nil },
	}
	d := newLifecycleDaemon(t, cfgMgr.config, fwd)
	d.cfgMgr = cfgMgr
	// 実際のフォワードマネージャーと同様、デーモンの ctx キャンセルで接続待ちが中断される
	fwd.startForwardFn = func(name string, _ core.CredentialCallback) error {
		if name == "done" {
			return nil
		}
		close(entered)
		<-d.ctx.Done()
		return context.Canceled
	}
	fwd.getAllSessionsFn = func() []core.ForwardSession {
		return []core.ForwardSession{{Status: core.Active, Rule: rules[0]}}
	}

	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	select {
	case <-entered:
	case <-time.After(lifecycleTestBound):
		t.Fatal("background restore did not reach the blocked rule")
	}

	stopWithin(t, d)

	if saved == nil {
		t.Fatal("SaveState was not called")
	}
	var got []string
	for _, r := range saved.ActiveForwards {
		got = append(got, r.Name)
	}
	if want := []string{"done", "dialing", "waiting"}; !slices.Equal(got, want) {
		t.Errorf("saved ActiveForwards = %v, want %v", got, want)
	}
}

// 復元に失敗したルールは（停止とは無関係なので）従来どおり保存対象から外れる。
func TestSaveState_DropsRestoreFailedRule(t *testing.T) {
	var saved *core.State
	fwd := &mockForwardManagerForState{startForwardFn: func(string, core.CredentialCallback) error {
		return &core.NotFoundError{Resource: "rule", Name: "gone"}
	}}
	cfgMgr := &mockConfigManagerForState{
		config: &core.Config{Session: core.SessionConfig{AutoRestore: true}},
		loadStateFn: func() (*core.State, error) {
			return &core.State{ActiveForwards: []core.ForwardRule{{Name: "gone"}}}, nil
		},
		saveStateFn: func(s *core.State) error { saved = s; return nil },
	}
	d := newDaemonForStateTestFull(cfgMgr, fwd)
	d.restoreState()
	d.saveState()
	if saved == nil || len(saved.ActiveForwards) != 0 {
		t.Errorf("saved = %+v, want no ActiveForwards", saved)
	}
}
