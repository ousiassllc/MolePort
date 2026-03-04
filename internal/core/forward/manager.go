package forward

import (
	"context"
	"fmt"
	"log/slog"
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
	mu          sync.RWMutex
	sshManager  core.SSHManager
	rules       map[string]core.ForwardRule
	ruleOrder   []string // 追加順序を保持
	active      map[string]*activeForward
	subscribers []chan core.ForwardEvent
	closed      bool
	nextID      int
}

// NewForwardManager は ForwardManager の実装を返す。
func NewForwardManager(sshManager core.SSHManager) core.ForwardManager {
	return &forwardManager{
		sshManager: sshManager,
		rules:      make(map[string]core.ForwardRule),
		active:     make(map[string]*activeForward),
	}
}

// AddRule はフォワーディングルールを追加する。
// 成功時はルール名（自動生成名を含む）を返す。
func (m *forwardManager) AddRule(rule core.ForwardRule) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 名前が空の場合は自動生成
	if rule.Name == "" {
		m.nextID++
		rule.Name = fmt.Sprintf("forward-%d", m.nextID)
	}

	// 名前の一意性チェック
	if _, exists := m.rules[rule.Name]; exists {
		return "", &core.AlreadyExistsError{Resource: "rule", Name: rule.Name}
	}

	// バリデーション
	if rule.Host == "" {
		return "", fmt.Errorf("host is required")
	}

	if rule.LocalPort < 1 || rule.LocalPort > 65535 {
		return "", fmt.Errorf("local_port must be between 1 and 65535, got %d", rule.LocalPort)
	}

	if rule.Type == core.Local || rule.Type == core.Remote {
		if rule.RemotePort < 1 || rule.RemotePort > 65535 {
			return "", fmt.Errorf("remote_port must be between 1 and 65535, got %d", rule.RemotePort)
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
	// ruleOrder から削除
	for i, n := range m.ruleOrder {
		if n == name {
			m.ruleOrder = append(m.ruleOrder[:i], m.ruleOrder[i+1:]...)
			break
		}
	}
	m.mu.Unlock()

	if session != nil {
		m.emit(core.ForwardEvent{
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

// emit はイベントを全サブスクライバーに非ブロッキングで送信する。
func (m *forwardManager) emit(event core.ForwardEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			slog.Warn("event dropped", "event_type", fmt.Sprintf("%T", event))
		}
	}
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

	ch := make(chan core.ForwardEvent, 16)
	m.subscribers = append(m.subscribers, ch)
	return ch
}
