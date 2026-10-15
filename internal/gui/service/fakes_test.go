package service

import (
	"sync"

	"github.com/ousiassllc/moleport/internal/core"
)

// fakeSSH は core.SSHManager の一部だけを実装するテスト用フェイク。
// 未実装のメソッドは nil の埋め込みインターフェースを呼ぶため、呼ばれるとパニックして検出できる。
type fakeSSH struct {
	core.SSHManager
	mu    sync.Mutex
	hosts []core.SSHHost
	ch    chan core.SSHEvent
}

func newFakeSSH(hosts ...core.SSHHost) *fakeSSH {
	return &fakeSSH{hosts: hosts, ch: make(chan core.SSHEvent, 16)}
}

func (f *fakeSSH) GetHosts() []core.SSHHost {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.SSHHost(nil), f.hosts...)
}

func (f *fakeSSH) Subscribe() <-chan core.SSHEvent { return f.ch }

// fakeForward は core.ForwardManager の一部だけを実装するテスト用フェイク。
type fakeForward struct {
	core.ForwardManager
	mu       sync.Mutex
	rules    []core.ForwardRule
	sessions []core.ForwardSession
	ch       chan core.ForwardEvent
}

func newFakeForward(sessions ...core.ForwardSession) *fakeForward {
	f := &fakeForward{ch: make(chan core.ForwardEvent, 16)}
	f.setSessions(sessions...)
	return f
}

// setSessions はセッションを差し替え、ルールをセッションのルールから導出する。
func (f *fakeForward) setSessions(sessions ...core.ForwardSession) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = sessions
	f.rules = nil
	for _, s := range sessions {
		f.rules = append(f.rules, s.Rule)
	}
}

func (f *fakeForward) GetRules() []core.ForwardRule {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.ForwardRule(nil), f.rules...)
}

func (f *fakeForward) GetAllSessions() []core.ForwardSession {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.ForwardSession(nil), f.sessions...)
}

func (f *fakeForward) Subscribe() <-chan core.ForwardEvent { return f.ch }

// staticBackend は固定の Backend を返す BackendSource。
type staticBackend struct {
	b  Backend
	ok bool
}

func (s staticBackend) Backend() (Backend, bool) { return s.b, s.ok }
