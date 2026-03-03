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
		return "", fmt.Errorf("rule %q already exists", rule.Name)
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
		return fmt.Errorf("rule %q not found", name)
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
		}
	}
}
