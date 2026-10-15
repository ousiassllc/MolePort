package bridge

import (
	"errors"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
)

func TestEventBridge_HiddenWindowSuppressesNotifyButKeepsDraining(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, false)

		// Subscribe チャネルは容量 16。受信ループが非表示中も即デキューするなら、
		// それを大きく超える数を送ってもブロックしない（Core のイベント破棄を招かない）。
		const total = 500
		sent := make(chan struct{})
		go func() {
			defer close(sent)
			for range total {
				e.fwd.Events <- core.ForwardEvent{Type: core.ForwardEventError, RuleName: "web", Error: errors.New("x")}
			}
		}()
		synctest.Wait()
		select {
		case <-sent:
		default:
			t.Fatal("非表示中に受信ループが詰まり、送信側がブロックしている")
		}

		time.Sleep(coalesceWindow * 2)
		synctest.Wait()

		if got := e.n.take(); len(got) != 0 {
			t.Errorf("非表示中に Notify してはならない: %v", got)
		}
		// 通知は非表示中も溜める（表示復帰後に取り出す）。
		if got := len(e.q.Drain()); got != service.MaxNotifications {
			t.Errorf("溜まった通知 = %d, want %d（上限）", got, service.MaxNotifications)
		}
		e.stop()
	})
}

func TestEventBridge_LiveTicker(t *testing.T) {
	tests := []struct {
		name     string
		visible  bool
		sessions []core.ForwardSession
		want     [][]service.Topic
	}{
		{"表示中かつアクティブ転送あり", true, []core.ForwardSession{activeSession("web")}, [][]service.Topic{{service.TopicForwards, service.TopicStatus}}},
		{"アクティブ転送なし", true, []core.ForwardSession{{Rule: core.ForwardRule{Name: "web"}, Status: core.Stopped}}, nil},
		{"非表示", false, []core.ForwardSession{activeSession("web")}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newBridgeEnv(t, tt.visible, tt.sessions...)

				time.Sleep(liveInterval + coalesceWindow)
				synctest.Wait()

				if got := e.n.take(); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("Notify = %v, want %v", got, tt.want)
				}
				e.stop()
			})
		})
	}
}

func TestEventBridge_ReconcileTickerNotifiesOnlyOnSignatureChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// 停止中のみ（ライブティッカーは動かない）。
		e := newBridgeEnv(t, true, core.ForwardSession{Rule: core.ForwardRule{Name: "web"}, Status: core.Stopped})

		time.Sleep(reconcileInterval + coalesceWindow)
		synctest.Wait()
		if got := e.n.take(); len(got) != 0 {
			t.Fatalf("変化なしでは通知しない: %v", got)
		}

		// CLI/TUI による forward.add の反映（ルールの増加）。
		e.fwd.SetSessions(
			core.ForwardSession{Rule: core.ForwardRule{Name: "web"}, Status: core.Stopped},
			core.ForwardSession{Rule: core.ForwardRule{Name: "added"}, Status: core.Stopped},
		)
		time.Sleep(reconcileInterval + coalesceWindow)
		synctest.Wait()
		want := [][]service.Topic{{service.TopicHosts, service.TopicForwards, service.TopicStatus}}
		if got := e.n.take(); !reflect.DeepEqual(got, want) {
			t.Fatalf("ルール追加: Notify = %v, want %v", got, want)
		}

		// 状態変化（Stopped → Error）でも通知する。
		e.fwd.SetSessions(
			core.ForwardSession{Rule: core.ForwardRule{Name: "web"}, Status: core.SessionError},
			core.ForwardSession{Rule: core.ForwardRule{Name: "added"}, Status: core.Stopped},
		)
		time.Sleep(reconcileInterval + coalesceWindow)
		synctest.Wait()
		if got := e.n.take(); !reflect.DeepEqual(got, want) {
			t.Fatalf("状態変化: Notify = %v, want %v", got, want)
		}

		// 以降は変化なしで通知しない。
		time.Sleep(reconcileInterval + coalesceWindow)
		synctest.Wait()
		if got := e.n.take(); len(got) != 0 {
			t.Errorf("変化なしでは通知しない: %v", got)
		}
		e.stop()
	})
}

func TestEventBridge_ReconcileTickerRunsWhileHiddenButDoesNotNotify(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, false)
		e.fwd.SetSessions(core.ForwardSession{Rule: core.ForwardRule{Name: "new"}, Status: core.Stopped})

		time.Sleep(reconcileInterval + coalesceWindow)
		synctest.Wait()
		if got := e.n.take(); len(got) != 0 {
			t.Errorf("非表示中は Notify しない: %v", got)
		}

		// 表示復帰後はシグネチャ変化済みのため再送されない（復帰時の all は Shell の責務）。
		e.vis.visible.Store(true)
		time.Sleep(reconcileInterval + coalesceWindow)
		synctest.Wait()
		if got := e.n.take(); len(got) != 0 {
			t.Errorf("シグネチャ変化がなければ通知しない: %v", got)
		}
		e.stop()
	})
}

func TestEventBridge_StopsOnContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, true, activeSession("web"))
		e.stop() // バブル内の全ゴルーチンが終了しなければ synctest がデッドロックを報告する

		e.ssh.Events <- core.SSHEvent{Type: core.SSHEventConnected}
		time.Sleep(time.Second)
		if got := e.n.take(); len(got) != 0 {
			t.Errorf("cancel 後に通知してはならない: %v", got)
		}
	})
}

func TestEventBridge_ClosedChannelsEndReceiveLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, true)
		close(e.ssh.Events)
		close(e.fwd.Events)
		synctest.Wait() // receive ゴルーチンが終了していること（emit ループのみ残る）
		e.stop()
	})
}
