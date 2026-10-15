// Package trayops はトレイメニューからの転送操作（UC-33）の Service。
package trayops

import (
	"errors"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// Actions はトレイメニューからの操作（UC-33）。ウィンドウを開かずに転送を操作する。
// 開始・停止は Forwards を経由するため、認証が必要な開始は CredentialBroker の前面化要求（UC-31）に進む。
type Actions struct {
	dash     service.Dashboard
	fwd      service.Forwards
	queue    *service.NotificationQueue
	notifier service.Notifier
}

// New は Actions を返す。dash は転送の現在状態（開始・停止のどちらが可能か）の判定に使う。
// 失敗は q にトーストとして積む（ウィンドウ非表示中も溜まり、表示時に取り出される）。
func New(dash service.Dashboard, fwd service.Forwards, q *service.NotificationQueue, n service.Notifier) *Actions {
	return &Actions{dash: dash, fwd: fwd, queue: q, notifier: n}
}

// ToggleForward は転送ルール name を、停止中なら開始、稼働中・開始中なら停止する。
func (t *Actions) ToggleForward(name string) {
	for _, v := range t.dash.Forwards() {
		if v.Name != name {
			continue
		}
		switch {
		case v.Actions.CanStop:
			t.report(name, t.fwd.Stop(name))
		case v.Actions.CanStart:
			t.report(name, t.fwd.Start(name))
		}
		t.notifier.Notify(service.TopicForwards, service.TopicHosts, service.TopicStatus)
		return
	}
}

// StopAll は停止可能な転送をすべて停止する。1 件の失敗で残りを止めない。
func (t *Actions) StopAll() {
	for _, v := range t.dash.Forwards() {
		if v.Actions.CanStop {
			t.report(v.Name, t.fwd.Stop(v.Name))
		}
	}
	t.notifier.Notify(service.TopicForwards, service.TopicHosts, service.TopicStatus)
}

// report は失敗をトーストに積む。状態が変わった直後の競合（ErrStateConflict）は無視する。
func (t *Actions) report(name string, err error) {
	if err == nil || errors.Is(err, service.ErrStateConflict) {
		return
	}
	t.queue.Push(service.ToastView{Kind: service.ToastError, Message: i18n.T("gui.tray.toast.failed", map[string]any{"Name": name, "Error": service.LocalizeError(err)})})
	t.notifier.Notify(service.TopicToast)
}
