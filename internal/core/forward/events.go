package forward

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/core"
)

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
		return nil, fmt.Errorf("rule %q not found", ruleName)
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
