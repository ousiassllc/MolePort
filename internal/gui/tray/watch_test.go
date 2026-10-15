package tray

import (
	"context"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

type fakeSSH struct {
	core.SSHManager
	ch chan core.SSHEvent
}

func (f *fakeSSH) Subscribe() <-chan core.SSHEvent { return f.ch }

type fakeFwd struct {
	core.ForwardManager
	ch chan core.ForwardEvent
}

func (f *fakeFwd) Subscribe() <-chan core.ForwardEvent { return f.ch }

func startWatch(t *testing.T, interval time.Duration) (*fakeSSH, *fakeFwd, chan struct{}, context.CancelFunc, chan struct{}) {
	t.Helper()
	ssh := &fakeSSH{ch: make(chan core.SSHEvent, 4)}
	fwd := &fakeFwd{ch: make(chan core.ForwardEvent, 4)}
	triggers := make(chan struct{}, 16)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() {
		Watch(ctx, ssh, fwd, interval, func() { triggers <- struct{}{} })
		close(returned)
	}()
	return ssh, fwd, triggers, cancel, returned
}

func waitTrigger(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("trigger が呼ばれなかった")
	}
}

func TestWatch_TriggersOnEvents(t *testing.T) {
	ssh, fwd, triggers, cancel, returned := startWatch(t, time.Hour)

	ssh.ch <- core.SSHEvent{}
	waitTrigger(t, triggers)
	fwd.ch <- core.ForwardEvent{}
	waitTrigger(t, triggers)

	cancel()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("ctx キャンセルで戻らなかった")
	}
}

func TestWatch_TriggersOnTicker(t *testing.T) {
	_, _, triggers, cancel, _ := startWatch(t, 10*time.Millisecond)
	defer cancel()

	waitTrigger(t, triggers)
}

func TestWatch_SurvivesClosedChannels(t *testing.T) {
	ssh, fwd, triggers, cancel, _ := startWatch(t, 10*time.Millisecond)
	defer cancel()

	close(ssh.ch)
	close(fwd.ch)

	// チャネルがクローズされてもティッカーで更新を続ける（busy loop しない）。
	waitTrigger(t, triggers)
	waitTrigger(t, triggers)
}
