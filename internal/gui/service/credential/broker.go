// Package credential は GUI のクレデンシャル要求（UC-31）を扱う CredentialBroker。
// Core の認証コールバックを実装し、WebView のダイアログ経由で値を受け取る。
package credential

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
)

// ErrUnknownRequest は存在しない・解決済み・タイムアウト済みのクレデンシャル要求 ID を表す。
var ErrUnknownRequest = errors.New("unknown credential request")

// WindowController はウィンドウを前面に出す。非表示（トレイ常駐）中にクレデンシャルが必要になったとき、
// ダイアログを見せるために呼ぶ（UC-31）。Shell が実装して注入する。
type WindowController interface {
	Show()
}

// Prompter は待機中のクレデンシャル要求を表示・解決する。*Broker が実装する。
type Prompter interface {
	Pending() (service.CredentialView, bool)
	Resolve(id string, in Input) error
	Cancel(id string) error
}

// Input は POST ボディから取り出したクレデンシャル値。
// 呼び出し側が確保したバッファのまま渡し、Resolve が Core へ引き渡した後にゼロ化する（NFR-36）。
type Input struct {
	Value   []byte   // password / passphrase
	Answers [][]byte // keyboard-interactive の各プロンプトへの回答
}

// Zero は保持しているバッファの内容をゼロで上書きする。
func (c Input) Zero() {
	clear(c.Value)
	for _, a := range c.Answers {
		clear(a)
	}
}

// Broker は core.CredentialCallback を実装し、WebView を介してクレデンシャルを要求する。
// IPC ハンドラの buildCredentialCallback と同じ契約で、通知先が WebView に変わるだけである。
//
// 要求ごとに待機チャネルを持ち、Resolve / Cancel / タイムアウト（core.CredentialTimeout）のいずれかで解放する。
// 複数の要求は FIFO で 1 件ずつ表示する。ログ・イベント・URL に値を含めない。
// ダッシュボードとトレイ（UC-33）からの接続・転送開始は同じ Broker の Callback を使う。
type Broker struct {
	notifier service.Notifier
	win      WindowController // nil 可
	timeout  time.Duration
	nextID   atomic.Int64

	mu      sync.Mutex
	pending []*pendingRequest
}

type pendingRequest struct {
	id     string
	req    core.CredentialRequest
	result chan core.CredentialResponse // 容量 1。Resolve / Cancel が 1 回だけ送る
}

var (
	_ Prompter                 = (*Broker)(nil)
	_ service.CallbackProvider = (*Broker)(nil)
)

// NewBroker は Broker を返す。win が nil の場合はウィンドウの前面化を行わない。
func NewBroker(n service.Notifier, win WindowController) *Broker {
	return &Broker{notifier: n, win: win, timeout: core.CredentialTimeout}
}

// Callback は Core に渡す認証コールバックを返す。
func (b *Broker) Callback() core.CredentialCallback { return b.request }

// request は要求を待機キューに積み、解決・キャンセル・タイムアウトのいずれかまでブロックする。
func (b *Broker) request(req core.CredentialRequest) (core.CredentialResponse, error) {
	p := &pendingRequest{
		id:     fmt.Sprintf("gcr-%d", b.nextID.Add(1)),
		req:    req,
		result: make(chan core.CredentialResponse, 1),
	}
	b.mu.Lock()
	b.pending = append(b.pending, p)
	b.mu.Unlock()

	if b.win != nil {
		b.win.Show()
	}
	b.notifier.Notify(service.TopicCredential)

	timer := time.NewTimer(b.timeout)
	defer timer.Stop()
	select {
	case resp := <-p.result:
		return finishRequest(resp)
	case <-timer.C:
		if b.take(p.id) {
			b.notifier.Notify(service.TopicCredential)
			return core.CredentialResponse{}, core.ErrCredentialTimeout
		}
		// 満了と同時に Resolve / Cancel が待機キューから取り出していた。その結果を採用する。
		return finishRequest(<-p.result)
	}
}

func finishRequest(resp core.CredentialResponse) (core.CredentialResponse, error) {
	if resp.Cancelled {
		return core.CredentialResponse{}, core.ErrCredentialCancelled
	}
	return resp, nil
}

// take は id の要求を待機キューから取り除く。既に取り除かれていれば false。
func (b *Broker) take(id string) bool {
	_, ok := b.takeRequest(id)
	return ok
}

func (b *Broker) takeRequest(id string) (*pendingRequest, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, p := range b.pending {
		if p.id == id {
			b.pending = append(b.pending[:i], b.pending[i+1:]...)
			return p, true
		}
	}
	return nil, false
}

// Pending は先頭（最も古い）の待機中要求を返す。なければ ok=false。
func (b *Broker) Pending() (service.CredentialView, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		return service.CredentialView{}, false
	}
	p := b.pending[0]
	v := service.CredentialView{
		RequestID: p.id,
		Host:      p.req.Host,
		Type:      string(p.req.Type),
		Prompt:    p.req.Prompt,
		Remaining: len(b.pending) - 1,
	}
	for _, pi := range p.req.Prompts {
		v.Prompts = append(v.Prompts, service.CredentialPromptView{Label: pi.Prompt, Echo: pi.Echo})
	}
	return v, true
}

// Resolve は id の要求に値を渡して解放する。未知・解決済み・タイムアウト済みは ErrUnknownRequest。
// keyboard-interactive の回答数はプロンプト数に揃える（不足は空、超過は切り捨て）。
// 戻る時点で in のバッファはゼロ化される。
func (b *Broker) Resolve(id string, in Input) error {
	defer in.Zero()
	p, ok := b.takeRequest(id)
	if !ok {
		return ErrUnknownRequest
	}
	resp := core.CredentialResponse{RequestID: id, Value: string(in.Value)}
	if len(p.req.Prompts) > 0 {
		resp.Answers = make([]string, len(p.req.Prompts))
		for i := range resp.Answers {
			if i < len(in.Answers) {
				resp.Answers[i] = string(in.Answers[i])
			}
		}
	}
	p.result <- resp
	b.notifier.Notify(service.TopicCredential)
	return nil
}

// Cancel は id の要求をキャンセルする（Core には core.ErrCredentialCancelled が返る）。
// 未知・解決済み・タイムアウト済みは ErrUnknownRequest。
func (b *Broker) Cancel(id string) error {
	p, ok := b.takeRequest(id)
	if !ok {
		return ErrUnknownRequest
	}
	p.result <- core.CredentialResponse{RequestID: id, Cancelled: true}
	b.notifier.Notify(service.TopicCredential)
	return nil
}
