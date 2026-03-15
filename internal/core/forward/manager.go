package forward

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/ousiassllc/moleport/internal/core"
)

// activeForward は実行中のフォワーディングセッションを保持する。
// starting が true の場合、起動処理中のプレースホルダーを表す。
type activeForward struct {
	session  core.ForwardSession
	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	sent     atomic.Int64
	received atomic.Int64
	starting bool
}

type forwardManager struct {
	mu         sync.RWMutex
	ctx        context.Context
	sshManager core.SSHManager
	rules      map[string]core.ForwardRule
	ruleOrder  []string // 追加順序を保持
	active     map[string]*activeForward
	events     core.EventEmitter[core.ForwardEvent]
	closed     bool
	nextID     int
}

// NewForwardManager は ForwardManager の実装を返す。
func NewForwardManager(ctx context.Context, sshManager core.SSHManager) core.ForwardManager {
	m := &forwardManager{
		ctx:        ctx,
		sshManager: sshManager,
		rules:      make(map[string]core.ForwardRule),
		active:     make(map[string]*activeForward),
	}
	m.events = core.NewEventEmitter[core.ForwardEvent](&m.mu)
	return m
}

// AddRule はフォワーディングルールを追加する。
// 成功時はルール名（自動生成名を含む）を返す。
func (m *forwardManager) AddRule(rule core.ForwardRule) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		m.nextID++
		rule.Name = fmt.Sprintf("forward-%d", m.nextID)
	}

	if _, exists := m.rules[rule.Name]; exists {
		return "", &core.AlreadyExistsError{Resource: "rule", Name: rule.Name}
	}

	if rule.Host == "" {
		return "", fmt.Errorf("host is required")
	}

	if err := core.ValidatePort(rule.LocalPort); err != nil {
		return "", fmt.Errorf("local_port: %w", err)
	}

	if rule.Type == core.Local || rule.Type == core.Remote {
		if err := core.ValidatePort(rule.RemotePort); err != nil {
			return "", fmt.Errorf("remote_port: %w", err)
		}
		if rule.RemoteHost == "" {
			rule.RemoteHost = "localhost"
		}
	}

	m.rules[rule.Name] = rule
	m.ruleOrder = append(m.ruleOrder, rule.Name)
	return rule.Name, nil
}

// DeleteRule はフォワーディングルールを削除する。アクティブな場合は停止する。
func (m *forwardManager) DeleteRule(name string) error {
	m.mu.Lock()
	if _, exists := m.rules[name]; !exists {
		m.mu.Unlock()
		return &core.NotFoundError{Resource: "rule", Name: name}
	}

	// アクティブな場合は停止（ロックを保持したまま）
	session := m.stopForwardLocked(name)

	delete(m.rules, name)
	for i, n := range m.ruleOrder {
		if n == name {
			m.ruleOrder = append(m.ruleOrder[:i], m.ruleOrder[i+1:]...)
			break
		}
	}
	m.mu.Unlock()

	if session != nil {
		m.events.Emit(core.ForwardEvent{
			Type:     core.ForwardEventStopped,
			RuleName: name,
			Session:  session,
		})
	}
	return nil
}

// GetRules は全ルールを追加順に返す。
func (m *forwardManager) GetRules() []core.ForwardRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]core.ForwardRule, 0, len(m.ruleOrder))
	for _, name := range m.ruleOrder {
		if rule, ok := m.rules[name]; ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetRulesByHost はホスト名でフィルタしたルール一覧を返す。
func (m *forwardManager) GetRulesByHost(hostName string) []core.ForwardRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []core.ForwardRule
	for _, name := range m.ruleOrder {
		if rule, ok := m.rules[name]; ok && rule.Host == hostName {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetSession はルール名からセッション情報を返す。
func (m *forwardManager) GetSession(ruleName string) (*core.ForwardSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if af, exists := m.active[ruleName]; exists && !af.starting {
		session := af.session
		session.BytesSent = af.sent.Load()
		session.BytesReceived = af.received.Load()
		return &session, nil
	}

	rule, exists := m.rules[ruleName]
	if !exists {
		return nil, &core.NotFoundError{Resource: "rule", Name: ruleName}
	}

	return &core.ForwardSession{
		Rule:   rule,
		Status: core.Stopped,
	}, nil
}

// GetAllSessions は全ルールのセッション情報を返す。
func (m *forwardManager) GetAllSessions() []core.ForwardSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]core.ForwardSession, 0, len(m.ruleOrder))
	for _, name := range m.ruleOrder {
		rule, ok := m.rules[name]
		if !ok {
			continue
		}

		if af, active := m.active[name]; active && !af.starting {
			session := af.session
			session.BytesSent = af.sent.Load()
			session.BytesReceived = af.received.Load()
			sessions = append(sessions, session)
		} else {
			sessions = append(sessions, core.ForwardSession{
				Rule:   rule,
				Status: core.Stopped,
			})
		}
	}
	return sessions
}

// Subscribe はイベントチャネルを返す。
func (m *forwardManager) Subscribe() <-chan core.ForwardEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.events.Subscribe()
}
