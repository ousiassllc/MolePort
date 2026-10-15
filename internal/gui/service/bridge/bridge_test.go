package bridge

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/servicetest"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// batchNotifier は Notify の呼び出しを 1 回 1 要素で記録する。
type batchNotifier struct {
	mu      sync.Mutex
	batches [][]service.Topic
}

func (n *batchNotifier) Notify(topics ...service.Topic) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.batches = append(n.batches, append([]service.Topic(nil), topics...))
}

func (n *batchNotifier) take() [][]service.Topic {
	n.mu.Lock()
	defer n.mu.Unlock()
	b := n.batches
	n.batches = nil
	return b
}

func useEnglish(t *testing.T) {
	t.Helper()
	prev := i18n.CurrentLang()
	t.Cleanup(func() { _ = i18n.SetLang(prev) })
	if err := i18n.SetLang(i18n.LangEN); err != nil {
		t.Fatalf("SetLang: %v", err)
	}
}

type fakeVisibility struct{ visible atomic.Bool }

func (f *fakeVisibility) Visible() bool { return f.visible.Load() }

type bridgeEnv struct {
	ssh    *servicetest.SSH
	fwd    *servicetest.Forward
	n      *batchNotifier
	q      *service.NotificationQueue
	vis    *fakeVisibility
	cancel context.CancelFunc
}

// newBridgeEnv は synctest のバブル内で EventBridge を起動する。
// 時間は偽クロックで進み、Sleep した分だけ即座にティッカー・タイマーが発火する。
func newBridgeEnv(t *testing.T, visible bool, sessions ...core.ForwardSession) *bridgeEnv {
	t.Helper()
	e := &bridgeEnv{
		ssh: servicetest.NewSSH(), fwd: servicetest.NewForward(sessions...),
		n: &batchNotifier{}, q: service.NewNotificationQueue(), vis: &fakeVisibility{},
	}
	e.vis.visible.Store(visible)
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	New(e.ssh, e.fwd, e.n, e.q, e.vis).Start(ctx)
	t.Cleanup(cancel)
	return e
}

func (e *bridgeEnv) stop() {
	e.cancel()
	synctest.Wait()
}

func activeSession(name string) core.ForwardSession {
	return core.ForwardSession{Rule: core.ForwardRule{Name: name, Host: "h"}, Status: core.Active}
}

func TestEventBridge_SSHEventMapsToHostsAndStatus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, true)
		e.ssh.Events <- core.SSHEvent{Type: core.SSHEventConnected, HostName: "h"}

		time.Sleep(coalesceWindow)
		synctest.Wait()

		want := [][]service.Topic{{service.TopicHosts, service.TopicStatus}}
		if got := e.n.take(); !reflect.DeepEqual(got, want) {
			t.Errorf("Notify = %v, want %v", got, want)
		}
		e.stop()
	})
}

func TestEventBridge_ForwardEventsMapToForwardsHostsStatus(t *testing.T) {
	for _, typ := range []core.ForwardEventType{
		core.ForwardEventStarted, core.ForwardEventStopped, core.ForwardEventReconnecting, core.ForwardEventRestored,
	} {
		t.Run(typ.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newBridgeEnv(t, true)
				e.fwd.Events <- core.ForwardEvent{Type: typ, RuleName: "web"}

				time.Sleep(coalesceWindow)
				synctest.Wait()

				want := [][]service.Topic{{service.TopicHosts, service.TopicForwards, service.TopicStatus}}
				if got := e.n.take(); !reflect.DeepEqual(got, want) {
					t.Errorf("Notify = %v, want %v", got, want)
				}
				if got := e.q.Drain(); len(got) != 0 {
					t.Errorf("エラー以外は通知キューに積まない: %+v", got)
				}
				e.stop()
			})
		})
	}
}

func TestEventBridge_CoalescesBurstIntoSingleNotify(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, true)
		for range 5 {
			e.ssh.Events <- core.SSHEvent{Type: core.SSHEventConnected, HostName: "h"}
			e.fwd.Events <- core.ForwardEvent{Type: core.ForwardEventStarted, RuleName: "web"}
			time.Sleep(10 * time.Millisecond)
		}
		synctest.Wait()
		if got := e.n.take(); len(got) != 0 {
			t.Fatalf("窓（100ms）が閉じる前に通知してはならない: %v", got)
		}

		time.Sleep(coalesceWindow)
		synctest.Wait()

		want := [][]service.Topic{{service.TopicHosts, service.TopicForwards, service.TopicStatus}}
		if got := e.n.take(); !reflect.DeepEqual(got, want) {
			t.Errorf("Notify = %v, want %v（1 回にまとめる）", got, want)
		}
		e.stop()
	})
}

func TestEventBridge_ErrorsPushToastAndNotifyToastTopic(t *testing.T) {
	useEnglish(t)
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, true)
		e.ssh.Events <- core.SSHEvent{Type: core.SSHEventError, HostName: "h", Error: errors.New("ssh: unable to authenticate")}
		e.fwd.Events <- core.ForwardEvent{Type: core.ForwardEventError, RuleName: "web", Error: errors.New("listen tcp :80: bind: address already in use")}

		time.Sleep(coalesceWindow)
		synctest.Wait()

		want := [][]service.Topic{{service.TopicHosts, service.TopicForwards, service.TopicStatus, service.TopicToast}}
		if got := e.n.take(); !reflect.DeepEqual(got, want) {
			t.Errorf("Notify = %v, want %v", got, want)
		}
		toasts := e.q.Drain()
		if len(toasts) != 2 {
			t.Fatalf("toasts = %+v, want 2 件", toasts)
		}
		// 2 つの購読チャネルの受信順は不定のため、集合として比較する。
		got := map[string]service.ToastKind{}
		for _, toast := range toasts {
			got[toast.Message] = toast.Kind
		}
		for _, msg := range []string{
			`Error on host "h": Authentication failed.`,
			`Error on forward "web": The port is already in use.`,
		} {
			if got[msg] != service.ToastError {
				t.Errorf("エラートースト %q がない: %+v", msg, toasts)
			}
		}
		e.stop()
	})
}

func TestEventBridge_NotifyCarriesOnlyTopicNames(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newBridgeEnv(t, true)
		e.ssh.Events <- core.SSHEvent{Type: core.SSHEventError, HostName: "secret-host", Error: errors.New("password=hunter2")}

		time.Sleep(coalesceWindow)
		synctest.Wait()

		valid := map[service.Topic]bool{}
		for _, topic := range topicOrder {
			valid[topic] = true
		}
		batches := e.n.take()
		if len(batches) == 0 {
			t.Fatal("通知がない")
		}
		for _, b := range batches {
			for _, topic := range b {
				if !valid[topic] {
					t.Errorf("トピック名以外が含まれる: %q", topic)
				}
				if strings.Contains(string(topic), "hunter2") || strings.Contains(string(topic), "secret-host") {
					t.Errorf("トピックにイベントのデータが含まれる: %q", topic)
				}
			}
		}
		e.stop()
	})
}
