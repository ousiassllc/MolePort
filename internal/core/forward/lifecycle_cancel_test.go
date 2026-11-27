package forward

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/forwardtest"
)

// startBlocker は StartForward を非同期で実行し、SSH 接続（または リスナー作成）の
// 段階でブロックさせるためのテスト用ハーネス。
type startBlocker struct {
	fm       *forwardManager
	entered  chan struct{} // ブロック地点に到達したら閉じられる
	release  chan struct{} // 閉じるとブロックが解除される
	listener *forwardtest.MockListener
	done     chan error // StartForward の戻り値
}

func newStartBlocker(t *testing.T, blockOnListener bool) *startBlocker {
	t.Helper()
	b := &startBlocker{
		entered:  make(chan struct{}),
		release:  make(chan struct{}),
		listener: forwardtest.NewMockListener(),
		done:     make(chan error, 1),
	}
	block := func() {
		close(b.entered)
		<-b.release
	}

	sm := forwardtest.NewMockSSHManager()
	conn := forwardtest.NewMockConn(false, false)
	conn.LocalForwardF = func(_ context.Context, _ int, _ string) (net.Listener, error) {
		if blockOnListener {
			block()
		}
		return b.listener, nil
	}
	if blockOnListener {
		sm.SetConnected("server1", conn)
	} else {
		sm.ConnectWithCbFn = func(hostName string, _ core.CredentialCallback) error {
			block()
			sm.SetConnected(hostName, conn)
			return nil
		}
	}

	b.fm = NewForwardManager(context.Background(), sm).(*forwardManager)
	if _, err := b.fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	}); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	go func() { b.done <- b.fm.StartForward("web", nil) }()
	select {
	case <-b.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for StartForward to reach blocking point")
	}
	return b
}

// finish はブロックを解除し、StartForward がキャンセルエラーで戻ることを検証する。
func (b *startBlocker) finish(t *testing.T) {
	t.Helper()
	close(b.release)
	select {
	case err := <-b.done:
		if err == nil {
			t.Error("StartForward() should fail when cancelled during start")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for StartForward to return")
	}
}

func (b *startBlocker) assertNothingLeft(t *testing.T) {
	t.Helper()
	b.fm.mu.RLock()
	n := len(b.fm.active)
	b.fm.mu.RUnlock()
	if n != 0 {
		t.Errorf("len(active) = %d, want 0", n)
	}
}

func (b *startBlocker) assertListenerClosed(t *testing.T) {
	t.Helper()
	select {
	case _, ok := <-b.listener.ConnCh:
		if ok {
			t.Error("unexpected connection on listener")
		}
	default:
		t.Error("listener should be closed")
	}
}

// Issue #105 の回帰テスト: 接続待ち中に StopForward しても転送が確立されない。
func TestForwardManager_StopForward_DuringConnect(t *testing.T) {
	b := newStartBlocker(t, false)
	if err := b.fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}
	b.finish(t)
	b.assertNothingLeft(t)
	forwardtest.AssertSessionStatus(t, b.fm, "web", core.Stopped)
}

// Issue #105 の回帰テスト: 接続待ち中に DeleteRule しても孤立した転送が残らない。
func TestForwardManager_DeleteRule_DuringConnect(t *testing.T) {
	b := newStartBlocker(t, false)
	if err := b.fm.DeleteRule("web"); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}
	b.finish(t)
	b.assertNothingLeft(t)
}

// Issue #105 の回帰テスト: リスナー作成中に DeleteRule してもリスナーが閉じられ、転送が残らない。
func TestForwardManager_DeleteRule_DuringListen(t *testing.T) {
	b := newStartBlocker(t, true)
	if err := b.fm.DeleteRule("web"); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}
	b.finish(t)
	b.assertNothingLeft(t)
	b.assertListenerClosed(t)
}

// Issue #105 の回帰テスト: リスナー作成中に StopForward してもリスナーが閉じられる。
func TestForwardManager_StopForward_DuringListen(t *testing.T) {
	b := newStartBlocker(t, true)
	if err := b.fm.StopForward("web"); err != nil {
		t.Fatalf("StopForward() error = %v", err)
	}
	b.finish(t)
	b.assertNothingLeft(t)
	b.assertListenerClosed(t)
}

// Issue #105 の回帰テスト: 開始処理中に Close しても転送が確立されない。
func TestForwardManager_Close_DuringStart(t *testing.T) {
	b := newStartBlocker(t, true)
	b.fm.Close()
	b.finish(t)
	b.assertNothingLeft(t)
	b.assertListenerClosed(t)
}

// Issue #105 の回帰テスト: Close 後の StartForward は失敗する。
func TestForwardManager_StartForward_AfterClose(t *testing.T) {
	sm := forwardtest.NewMockSSHManager()
	sm.SetConnected("server1", forwardtest.NewMockConn(true, false))
	fm := NewForwardManager(context.Background(), sm)
	_, _ = fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	fm.Close()
	if err := fm.StartForward("web", nil); err == nil {
		t.Fatal("StartForward() should fail after Close()")
	}
}
