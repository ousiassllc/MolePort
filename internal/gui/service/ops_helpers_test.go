package service

import (
	"slices"
	"sync"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service/servicetest"
)

// syncNotifier は並行して呼ばれる Notify のトピックを記録する。
type syncNotifier struct {
	mu     sync.Mutex
	topics []Topic
}

func (n *syncNotifier) Notify(topics ...Topic) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.topics = append(n.topics, topics...)
}

func (n *syncNotifier) has(t Topic) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Contains(n.topics, t)
}

// stubCallback は固定の認証コールバックを提供する CallbackProvider。
type stubCallback struct{}

func (stubCallback) Callback() core.CredentialCallback {
	return func(core.CredentialRequest) (core.CredentialResponse, error) {
		return core.CredentialResponse{}, core.ErrCredentialCancelled
	}
}

// opsEnv は HostService / ForwardService / DashboardService を同じ偽 Core と Activity で組み立てる。
type opsEnv struct {
	ssh   *servicetest.SSH
	fwd   *servicetest.Forward
	cfg   *servicetest.Config
	q     *NotificationQueue
	n     *syncNotifier
	hosts *HostService
	fwds  *ForwardService
	dash  *DashboardService
}

func newOpsEnv(hosts []core.SSHHost, sessions ...core.ForwardSession) *opsEnv {
	e := &opsEnv{
		ssh: servicetest.NewSSH(hosts...),
		fwd: servicetest.NewForward(sessions...),
		cfg: &servicetest.Config{},
		q:   NewNotificationQueue(),
		n:   &syncNotifier{},
	}
	src := staticBackend{ok: true, b: Backend{SSH: e.ssh, Forward: e.fwd, Config: e.cfg}}
	act := NewActivity()
	e.hosts = NewHostService(src, stubCallback{}, act, e.q, e.n)
	e.fwds = NewForwardService(src, stubCallback{}, act, e.q, e.n)
	e.dash = NewDashboardService(src, act)
	return e
}

func (e *opsEnv) forwardView(name string) ForwardView {
	for _, v := range e.dash.Forwards() {
		if v.Name == name {
			return v
		}
	}
	return ForwardView{}
}

func (e *opsEnv) hostView(name string) HostView {
	for _, v := range e.dash.Hosts() {
		if v.Name == name {
			return v
		}
	}
	return HostView{}
}

func session(name, host string, st core.SessionStatus) core.ForwardSession {
	return core.ForwardSession{
		Rule:   core.ForwardRule{Name: name, Host: host, Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
		Status: st,
	}
}
