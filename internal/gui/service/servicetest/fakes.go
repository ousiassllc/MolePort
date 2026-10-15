// Package servicetest は GUI Service 層のテスト用に、Core（SSHManager / ForwardManager / ConfigManager）の
// 必要なメソッドだけを実装したフェイクを提供する。
// 未実装のメソッドは nil の埋め込みインターフェースを呼ぶため、呼ばれるとパニックして検出できる。
package servicetest

import (
	"fmt"
	"sync"

	"github.com/ousiassllc/moleport/internal/core"
)

// SSH は core.SSHManager のフェイク。
type SSH struct {
	core.SSHManager
	Events    chan core.SSHEvent                                  // Subscribe が返すチャネル（容量 16）
	ConnectFn func(name string, cb core.CredentialCallback) error // nil なら成功して Connected

	mu          sync.Mutex
	hosts       []core.SSHHost
	connects    []string
	disconnects []string
}

// NewSSH は hosts を持つ SSH を返す。
func NewSSH(hosts ...core.SSHHost) *SSH {
	return &SSH{hosts: hosts, Events: make(chan core.SSHEvent, 16)}
}

func (f *SSH) GetHosts() []core.SSHHost {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.SSHHost(nil), f.hosts...)
}

func (f *SSH) GetHost(name string) (*core.SSHHost, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.hosts {
		if f.hosts[i].Name == name {
			h := f.hosts[i]
			return &h, nil
		}
	}
	return nil, &core.NotFoundError{Resource: "host", Name: name}
}

func (f *SSH) Subscribe() <-chan core.SSHEvent { return f.Events }

// SetState はホストの接続状態を変える。
func (f *SSH) SetState(name string, s core.ConnectionState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.hosts {
		if f.hosts[i].Name == name {
			f.hosts[i].State = s
		}
	}
}

// ConnectWithCallback は ConnectFn（なければ成功して Connected）を実行する。
func (f *SSH) ConnectWithCallback(name string, cb core.CredentialCallback) error {
	f.mu.Lock()
	fn := f.ConnectFn
	f.connects = append(f.connects, name)
	f.mu.Unlock()
	if fn != nil {
		return fn(name, cb)
	}
	f.SetState(name, core.Connected)
	return nil
}

func (f *SSH) Disconnect(name string) error {
	f.mu.Lock()
	f.disconnects = append(f.disconnects, name)
	f.mu.Unlock()
	f.SetState(name, core.Disconnected)
	return nil
}

// Connected は ConnectWithCallback が呼ばれたホスト名を呼び出し順に返す。
func (f *SSH) Connected() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.connects...)
}

// Forward は core.ForwardManager のフェイク。ルールはセッションと 1 対 1 で保持する。
type Forward struct {
	core.ForwardManager
	Events  chan core.ForwardEvent                              // Subscribe が返すチャネル（容量 16）
	StartFn func(name string, cb core.CredentialCallback) error // nil なら成功して Active

	mu       sync.Mutex
	sessions []core.ForwardSession
	starts   []string
	stops    []string
}

// NewForward は sessions を持つ Forward を返す。
func NewForward(sessions ...core.ForwardSession) *Forward {
	f := &Forward{Events: make(chan core.ForwardEvent, 16)}
	f.SetSessions(sessions...)
	return f
}

// SetSessions はセッション（とそのルール）を差し替える。
func (f *Forward) SetSessions(sessions ...core.ForwardSession) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = append([]core.ForwardSession(nil), sessions...)
}

func (f *Forward) GetRules() []core.ForwardRule {
	f.mu.Lock()
	defer f.mu.Unlock()
	rules := make([]core.ForwardRule, len(f.sessions))
	for i, s := range f.sessions {
		rules[i] = s.Rule
	}
	return rules
}

func (f *Forward) GetRulesByHost(host string) []core.ForwardRule {
	var out []core.ForwardRule
	for _, r := range f.GetRules() {
		if r.Host == host {
			out = append(out, r)
		}
	}
	return out
}

func (f *Forward) GetAllSessions() []core.ForwardSession {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.ForwardSession(nil), f.sessions...)
}

func (f *Forward) GetSession(name string) (*core.ForwardSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.sessions {
		if f.sessions[i].Rule.Name == name {
			s := f.sessions[i]
			return &s, nil
		}
	}
	return nil, &core.NotFoundError{Resource: "rule", Name: name}
}

func (f *Forward) Subscribe() <-chan core.ForwardEvent { return f.Events }

// AddRule は名前が空なら forward-N を割り当て、重複は core.AlreadyExistsError を返す。
func (f *Forward) AddRule(rule core.ForwardRule) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rule.Name == "" {
		rule.Name = fmt.Sprintf("forward-%d", len(f.sessions)+1)
	}
	for _, s := range f.sessions {
		if s.Rule.Name == rule.Name {
			return "", &core.AlreadyExistsError{Resource: "rule", Name: rule.Name}
		}
	}
	f.sessions = append(f.sessions, core.ForwardSession{Rule: rule, Status: core.Stopped})
	return rule.Name, nil
}

func (f *Forward) DeleteRule(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.sessions {
		if s.Rule.Name == name {
			f.sessions = append(f.sessions[:i], f.sessions[i+1:]...)
			return nil
		}
	}
	return &core.NotFoundError{Resource: "rule", Name: name}
}

// SetStatus はセッションの状態を変える。
func (f *Forward) SetStatus(name string, st core.SessionStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.sessions {
		if f.sessions[i].Rule.Name == name {
			f.sessions[i].Status = st
		}
	}
}

// StartForward は StartFn（なければ成功して Active）を実行する。
func (f *Forward) StartForward(name string, cb core.CredentialCallback) error {
	f.mu.Lock()
	fn := f.StartFn
	f.starts = append(f.starts, name)
	f.mu.Unlock()
	if fn != nil {
		return fn(name, cb)
	}
	f.SetStatus(name, core.Active)
	return nil
}

func (f *Forward) StopForward(name string) error {
	f.mu.Lock()
	f.stops = append(f.stops, name)
	f.mu.Unlock()
	f.SetStatus(name, core.Stopped)
	return nil
}

// Started は StartForward が呼ばれたルール名を呼び出し順に返す。
func (f *Forward) Started() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.starts...)
}

// Stopped は StopForward が呼ばれたルール名を呼び出し順に返す。
func (f *Forward) Stopped() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.stops...)
}

// Config は core.ConfigManager のフェイク。UpdateConfig で保存された転送ルールを記録する。
// Err を設定すると UpdateConfig がそのエラーを返す。
type Config struct {
	core.ConfigManager
	Err error

	mu    sync.Mutex
	saved []core.ForwardRule
	saves int
}

func (c *Config) UpdateConfig(fn func(*core.Config)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Err != nil {
		return c.Err
	}
	var cfg core.Config
	fn(&cfg)
	c.saved = cfg.Forwards
	c.saves++
	return nil
}

// SavedRules は最後に保存された転送ルールを返す。
func (c *Config) SavedRules() []core.ForwardRule {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]core.ForwardRule(nil), c.saved...)
}

// Saves は UpdateConfig が成功した回数を返す。
func (c *Config) Saves() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saves
}
