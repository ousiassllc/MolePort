package service

import (
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// suggestedPorts はポート競合時に提示する空きポート候補の最大数。
const suggestedPorts = 3

// CallbackProvider は Core に渡す認証コールバックを提供する。credential.Broker が実装する。
// 接続・転送開始（ダッシュボード・トレイ）は同じコールバックを使う（UC-33）。
type CallbackProvider interface {
	Callback() core.CredentialCallback
}

// operations は HostService と ForwardService が共有する、Core を非同期に操作するための依存と処理。
type operations struct {
	src      BackendSource
	cb       CallbackProvider // 認証コールバックの提供元（credential.Broker）
	act      *Activity
	queue    *NotificationQueue
	notifier Notifier
	portFree func(port int) bool // 候補ポートの空き確認。テストで差し替える
}

func newOperations(src BackendSource, cb CallbackProvider, act *Activity, q *NotificationQueue, n Notifier) *operations {
	return &operations{src: src, cb: cb, act: act, queue: q, notifier: n, portFree: localPortFree}
}

// backend は起動済みの Backend を返す。内蔵デーモンの起動前・失敗時は ErrNotReady。
func (o *operations) backend() (Backend, error) {
	b, ok := o.src.Backend()
	if !ok {
		return Backend{}, ErrNotReady
	}
	return b, nil
}

// startAsync は転送ルールの開始を進行中として記録し、認証待ちでブロックしないよう別ゴルーチンで Core を呼ぶ。
// 既に開始処理中なら何もしない。失敗は Error 状態（Activity）とトーストで通知する。
// 呼び出し元は事前に状態（CanStart）を検査しておくこと。
func (o *operations) startAsync(b Backend, rule core.ForwardRule) {
	if !o.act.beginStart(rule.Name) {
		return
	}
	go o.runStart(b, rule)
}

// runStart は StartForward を同期的に実行して結果を記録する。
func (o *operations) runStart(b Backend, rule core.ForwardRule) {
	err := b.Forward.StartForward(rule.Name, o.cb.Callback())
	if err == nil {
		o.act.endStart(rule.Name, "")
		o.notifier.Notify(TopicForwards, TopicHosts, TopicStatus)
		return
	}
	slog.Debug("gui: start forward failed", "rule", rule.Name, "error", err)
	msg := o.startFailureMessage(rule, err)
	o.act.endStart(rule.Name, msg)
	o.queue.Push(ToastView{Kind: ToastError, Message: i18n.T("gui.forward.toast.start_failed", map[string]any{"Name": rule.Name, "Error": msg})})
	o.notifier.Notify(TopicForwards, TopicHosts, TopicStatus, TopicToast)
}

// startFailureMessage は転送開始の失敗をローカライズした説明にする。
// ポート競合（ローカル側で待ち受けるルール）には空きポートの候補を添える（UC-28）。
func (o *operations) startFailureMessage(rule core.ForwardRule, err error) string {
	if !isPortInUse(err) || rule.Type == core.Remote {
		return LocalizeError(err)
	}
	ports := o.suggestPorts(rule.LocalPort)
	if len(ports) == 0 {
		return LocalizeError(err)
	}
	return i18n.T("gui.error.port_in_use_suggest", map[string]any{"Ports": strings.Join(ports, ", ")})
}

// suggestPorts は port より大きい空きポートを最大 suggestedPorts 件返す。
func (o *operations) suggestPorts(port int) []string {
	var ports []string
	for p := port + 1; p <= core.MaxPort && len(ports) < suggestedPorts; p++ {
		if o.portFree(p) {
			ports = append(ports, strconv.Itoa(p))
		}
	}
	return ports
}

// localPortFree は 127.0.0.1 で port を待ち受けられるかを確認する（待ち受けてすぐ閉じる）。
func localPortFree(port int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}
