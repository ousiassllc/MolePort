package daemon

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/daemon/pidfile"
)

// lifecycleTestBound はチャネル待ちがテストを固めないための上限。通常は使われない。
const lifecycleTestBound = 5 * time.Second

// lifecycleFwdMgr は Subscribe したチャネルを Close で閉じる ForwardManager のテスト用実装。
// closeFn で Close の挙動（ハングなど）を差し替えられる。
type lifecycleFwdMgr struct {
	mockForwardManagerForState
	closeFn func()
	once    sync.Once
	ch      chan core.ForwardEvent
}

func (m *lifecycleFwdMgr) Subscribe() <-chan core.ForwardEvent {
	m.ch = make(chan core.ForwardEvent, 1)
	return m.ch
}

func (m *lifecycleFwdMgr) Close() {
	if m.closeFn != nil {
		m.closeFn()
	}
	m.once.Do(func() { close(m.ch) })
}

// newLifecycleDaemon は実ソケット・PID ファイルを持つ Daemon を作り、
// 設定とフォワードマネージャーをテスト用に差し替える。
func newLifecycleDaemon(t *testing.T, cfg *core.Config, fwd core.ForwardManager) *Daemon {
	t.Helper()
	d, err := New(createTestConfigDir(t), "test")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	d.cfgMgr = &mockConfigManagerForState{config: cfg}
	d.fwdMgr = fwd
	return d
}

func stopWithin(t *testing.T, d *Daemon) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- d.Stop() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() error: %v", err)
		}
	case <-time.After(lifecycleTestBound):
		t.Fatal("Stop() did not return in time")
	}
}

// 応答しない SSH ホストの auto_connect ルールがあっても Start は即座に返り、
// Stop で復元 goroutine が回収される。
func TestDaemon_Start_RestoresInBackground(t *testing.T) {
	entered := make(chan struct{})
	fwd := &lifecycleFwdMgr{}
	cfg := &core.Config{Forwards: []core.ForwardRule{{Name: "slow", Host: "unreachable", AutoConnect: true}}}
	d := newLifecycleDaemon(t, cfg, fwd)
	// 実際のフォワードマネージャーと同様、デーモンの ctx キャンセルで接続待ちが中断される
	fwd.startForwardFn = func(string, core.CredentialCallback) error {
		close(entered)
		<-d.ctx.Done()
		return context.Canceled
	}
	// リークしていれば wg.Wait で Stop が止まり、強制切断のタイムアウトまで待たされる
	d.shutdownTimeout = time.Minute

	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	// Start が返った時点で、復元はまだ接続待ち（バックグラウンド）にいる
	select {
	case <-entered:
	case <-time.After(lifecycleTestBound):
		t.Fatal("background auto-start did not begin")
	}

	stopWithin(t, d)
}

// 切断処理がハングしても Stop は戻り、ソケットと PID ファイルが解放される。
func TestDaemon_Stop_ForcesCloseOnTimeout(t *testing.T) {
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })
	fwd := &lifecycleFwdMgr{closeFn: func() { <-hang }}
	d := newLifecycleDaemon(t, &core.Config{}, fwd)
	d.shutdownTimeout = 50 * time.Millisecond

	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	stopWithin(t, d)

	if _, err := os.Stat(SocketPath(d.configDir)); !os.IsNotExist(err) {
		t.Errorf("socket file still exists after Stop: %v", err)
	}
	if locked, _ := pidfile.IsLocked(PIDFilePath(d.configDir)); locked {
		t.Error("pid file is still locked after Stop")
	}
}

func TestDefaultShutdownTimeout(t *testing.T) {
	if defaultShutdownTimeout != 5*time.Second {
		t.Errorf("defaultShutdownTimeout = %v, want 5s (NFR-16)", defaultShutdownTimeout)
	}
}
