// Package bridge は Core のイベントとティッカーを WebView への更新トピックに変換する EventBridge。
package bridge

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// WindowVisibility はウィンドウの表示状態。Shell の VisibilityNotifier が満たす。
type WindowVisibility interface {
	Visible() bool
}

// EventBridge の既定の周期（docs/architecture/gui.md「EventBridge」）。
const (
	coalesceWindow    = 100 * time.Millisecond // 同一トピックを 1 回の通知にまとめる窓
	liveInterval      = time.Second            // 稼働時間・送受信量の更新（表示中かつアクティブ転送 ≥ 1）
	reconcileInterval = 5 * time.Second        // CLI/TUI による変更の照合（常時）
)

// bridgeTiming は EventBridge の周期。テストで短縮できるよう分離している。
type bridgeTiming struct {
	coalesce, live, reconcile time.Duration
}

// topicOrder は通知するトピックの出力順（出力を決定的にする）。
var topicOrder = []service.Topic{
	service.TopicHosts, service.TopicForwards, service.TopicStatus, service.TopicHeader,
	service.TopicCredential, service.TopicToast, service.TopicSettings, service.TopicAll,
}

// EventBridge は Core のイベントとティッカーをトピックに変換し、Notifier へ通知する。
//
// 受信ループ（receive）は Subscribe チャネル（バッファ 16。満杯で Core がイベントを破棄する）を
// 常に全件即デキューし、トピック集合へ写像するだけでブロックしない。
// コアレス・非表示中の抑止・Notifier 呼び出しはすべて下流の emit ループで行う。
type EventBridge struct {
	ssh      core.SSHManager
	fwd      core.ForwardManager
	notifier service.Notifier
	queue    *service.NotificationQueue
	win      WindowVisibility
	timing   bridgeTiming

	mu      sync.Mutex
	pending map[service.Topic]struct{}
	wake    chan struct{} // pending が空から非空になったことを emit ループへ知らせる（容量 1）
}

// New は EventBridge を返す。Start するまで何も購読しない。
func New(ssh core.SSHManager, fwd core.ForwardManager, n service.Notifier, q *service.NotificationQueue, win WindowVisibility) *EventBridge {
	return newEventBridge(ssh, fwd, n, q, win, bridgeTiming{coalesceWindow, liveInterval, reconcileInterval})
}

func newEventBridge(ssh core.SSHManager, fwd core.ForwardManager, n service.Notifier, q *service.NotificationQueue, win WindowVisibility, t bridgeTiming) *EventBridge {
	return &EventBridge{
		ssh: ssh, fwd: fwd, notifier: n, queue: q, win: win, timing: t,
		pending: make(map[service.Topic]struct{}),
		wake:    make(chan struct{}, 1),
	}
}

// Start は SSHManager / ForwardManager を購読し、受信ループと emit ループを起動する。
// ctx のキャンセルで両ループが終了する。
func (e *EventBridge) Start(ctx context.Context) {
	sshCh := e.ssh.Subscribe()
	fwdCh := e.fwd.Subscribe()
	sig := e.signature()
	go e.receive(ctx, sshCh, fwdCh)
	go e.emit(ctx, sig)
}

// receive は両チャネルを常に即デキューし、トピック集合へ写像する。
func (e *EventBridge) receive(ctx context.Context, sshCh <-chan core.SSHEvent, fwdCh <-chan core.ForwardEvent) {
	for sshCh != nil || fwdCh != nil {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sshCh:
			if !ok {
				sshCh = nil
				continue
			}
			e.handleSSH(ev)
		case ev, ok := <-fwdCh:
			if !ok {
				fwdCh = nil
				continue
			}
			e.handleForward(ev)
		}
	}
}

// handleSSH は SSHEvent（全種別）を hosts / status に写像する。エラーは通知キューにも積む。
func (e *EventBridge) handleSSH(ev core.SSHEvent) {
	e.add(service.TopicHosts, service.TopicStatus)
	if ev.Type != core.SSHEventError {
		return
	}
	e.queue.Push(service.ToastView{
		Kind: service.ToastError,
		Message: i18n.T("gui.toast.ssh_error", map[string]any{
			"Host":  ev.HostName,
			"Error": service.LocalizeError(ev.Error),
		}),
	})
	e.add(service.TopicToast)
}

// handleForward は ForwardEvent を forwards / hosts（アクティブ転送数）/ status に写像する。
// エラーは通知キューにも積む。
func (e *EventBridge) handleForward(ev core.ForwardEvent) {
	e.add(service.TopicForwards, service.TopicHosts, service.TopicStatus)
	if ev.Type != core.ForwardEventError {
		return
	}
	detail := service.LocalizeError(ev.Error)
	if detail == "" && ev.Session != nil {
		detail = ev.Session.LastError
	}
	e.queue.Push(service.ToastView{
		Kind: service.ToastError,
		Message: i18n.T("gui.toast.forward_error", map[string]any{
			"Name":  ev.RuleName,
			"Error": detail,
		}),
	})
	e.add(service.TopicToast)
}

// add はトピックを送信待ちに追加し、emit ループを起こす。ブロックしない。
func (e *EventBridge) add(topics ...service.Topic) {
	e.mu.Lock()
	for _, t := range topics {
		e.pending[t] = struct{}{}
	}
	e.mu.Unlock()
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// take は送信待ちのトピックを出力順に取り出して空にする。
func (e *EventBridge) take() []service.Topic {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.pending) == 0 {
		return nil
	}
	topics := make([]service.Topic, 0, len(e.pending))
	for _, t := range topicOrder {
		if _, ok := e.pending[t]; ok {
			topics = append(topics, t)
		}
	}
	clear(e.pending)
	return topics
}

// emit はコアレス窓・ライブ / 照合ティッカーを駆動し、窓の終わりに Notifier へ通知する。
func (e *EventBridge) emit(ctx context.Context, lastSig uint64) {
	live := time.NewTicker(e.timing.live)
	defer live.Stop()
	reconcile := time.NewTicker(e.timing.reconcile)
	defer reconcile.Stop()

	var window *time.Timer
	var windowC <-chan time.Time // 窓が開いていない間は nil（select で選ばれない）
	defer func() {
		if window != nil {
			window.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.wake:
			if window == nil {
				window = time.NewTimer(e.timing.coalesce)
				windowC = window.C
			}
		case <-windowC:
			window, windowC = nil, nil
			e.flush()
		case <-live.C:
			if e.win.Visible() && e.hasActiveForward() {
				e.add(service.TopicForwards, service.TopicStatus)
			}
		case <-reconcile.C:
			if sig := e.signature(); sig != lastSig {
				lastSig = sig
				e.add(service.TopicForwards, service.TopicHosts, service.TopicStatus)
			}
		}
	}
}

// flush は溜まったトピックを 1 回の Notify にまとめる。非表示中は送らず破棄する
// （表示復帰時に Shell が all を 1 回送って再同期する。NFR-33）。
func (e *EventBridge) flush() {
	topics := e.take()
	if len(topics) == 0 || !e.win.Visible() {
		return
	}
	e.notifier.Notify(topics...)
}

func (e *EventBridge) hasActiveForward() bool {
	for _, s := range e.fwd.GetAllSessions() {
		if s.Status == core.Active {
			return true
		}
	}
	return false
}

// signature はルール名とセッションの名前・状態から求めるシグネチャ。
// Core のメモリ上の状態だけを読み、ディスク・ネットワークにはアクセスしない。
func (e *EventBridge) signature() uint64 {
	h := fnv.New64a()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	for _, r := range e.fwd.GetRules() {
		write(r.Name)
	}
	write("|")
	for _, s := range e.fwd.GetAllSessions() {
		write(s.Rule.Name)
		write(s.Status.String())
	}
	return h.Sum64()
}
