package service

import "sync"

// Activity は進行中の非同期操作（接続・転送開始）と直近の転送開始失敗を保持し、
// Core の状態に重ねて表示するために使う。
//
// 接続・転送開始は認証待ちでブロックしないよう非同期に Core を呼ぶため、アクション直後のレスポンスでは
// Core がまだ Connecting / Starting に遷移していない。また転送開始の失敗は Core ではセッションが
// Stopped に戻るだけで Error として残らない。これらを ViewModel で補う。
// nil の *Activity は「何も進行していない」として扱う。
type Activity struct {
	mu         sync.Mutex
	connecting map[string]struct{}
	starting   map[string]struct{}
	failures   map[string]string
}

// NewActivity は空の Activity を返す。
func NewActivity() *Activity {
	return &Activity{
		connecting: make(map[string]struct{}),
		starting:   make(map[string]struct{}),
		failures:   make(map[string]string),
	}
}

// beginConnect は host の接続を進行中にする。既に進行中なら false を返す。
func (a *Activity) beginConnect(host string) bool { return a.begin(a.connecting, host) }

func (a *Activity) endConnect(host string) { a.end(a.connecting, host) }

// beginStart は転送ルールの開始を進行中にし、直近の失敗表示を消す。既に進行中なら false を返す。
func (a *Activity) beginStart(rule string) bool {
	if !a.begin(a.starting, rule) {
		return false
	}
	a.clearFailure(rule)
	return true
}

// endStart は転送ルールの開始を完了にする。failure が空でなければ失敗として記録する。
func (a *Activity) endStart(rule, failure string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.starting, rule)
	if failure != "" {
		a.failures[rule] = failure
	}
}

func (a *Activity) clearFailure(rule string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.failures, rule)
}

// Connecting は host の接続が進行中かを返す。
func (a *Activity) Connecting(host string) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.connecting[host]
	return ok
}

// Starting は転送ルールの開始が進行中かを返す。
func (a *Activity) Starting(rule string) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.starting[rule]
	return ok
}

// Failure は転送ルールの直近の開始失敗（ローカライズ済みメッセージ）を返す。なければ空。
func (a *Activity) Failure(rule string) string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.failures[rule]
}

func (a *Activity) begin(set map[string]struct{}, key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, busy := set[key]; busy {
		return false
	}
	set[key] = struct{}{}
	return true
}

func (a *Activity) end(set map[string]struct{}, key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(set, key)
}
