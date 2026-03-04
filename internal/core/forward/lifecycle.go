package forward

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// StartForward はフォワーディングセッションを開始する。
// cb が非 nil の場合、SSH 接続にクレデンシャルコールバックを使用する。
func (m *forwardManager) StartForward(ruleName string, cb core.CredentialCallback) error {
	m.mu.Lock()
	rule, exists := m.rules[ruleName]
	if !exists {
		m.mu.Unlock()
		return &core.NotFoundError{Resource: "rule", Name: ruleName}
	}

	if _, active := m.active[ruleName]; active {
		m.mu.Unlock()
		return &core.AlreadyActiveError{Name: ruleName}
	}

	m.active[ruleName] = &activeForward{starting: true}
	m.mu.Unlock()

	cleanup := func() {
		m.mu.Lock()
		if af, ok := m.active[ruleName]; ok && af.starting {
			delete(m.active, ruleName)
		}
		m.mu.Unlock()
	}

	if !m.sshManager.IsConnected(rule.Host) {
		if err := m.sshManager.ConnectWithCallback(rule.Host, cb); err != nil {
			cleanup()
			return fmt.Errorf("failed to connect to host %s: %w", rule.Host, err)
		}
	}

	sshConn, err := m.sshManager.GetSSHConnection(rule.Host)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to get SSH connection: %w", err)
	}

	sshClient, err := m.sshManager.GetConnection(rule.Host)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to get SSH client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var listener net.Listener
	switch rule.Type {
	case core.Local:
		remoteAddr := fmt.Sprintf("%s:%d", rule.RemoteHost, rule.RemotePort)
		listener, err = sshConn.LocalForward(ctx, rule.LocalPort, remoteAddr)
	case core.Remote:
		localAddr := fmt.Sprintf("127.0.0.1:%d", rule.LocalPort)
		listener, err = sshConn.RemoteForward(ctx, rule.RemotePort, localAddr)
	case core.Dynamic:
		listener, err = sshConn.DynamicForward(ctx, rule.LocalPort)
	default:
		cancel()
		cleanup()
		return fmt.Errorf("unsupported forward type: %v", rule.Type)
	}

	if err != nil {
		cancel()
		cleanup()
		return fmt.Errorf("failed to create listener: %w", err)
	}

	af := &activeForward{
		session: core.ForwardSession{
			ID:          fmt.Sprintf("%s-%d", ruleName, time.Now().UnixNano()),
			Rule:        rule,
			Status:      core.Active,
			ConnectedAt: time.Now(),
		},
		listener: listener,
		ctx:      ctx,
		cancel:   cancel,
	}

	m.mu.Lock()
	m.active[ruleName] = af
	m.mu.Unlock()

	go m.acceptLoop(af, rule, sshClient)

	m.emit(core.ForwardEvent{
		Type:     core.ForwardEventStarted,
		RuleName: ruleName,
		Session:  &af.session,
	})

	slog.Info("forward started", "rule", ruleName, "type", rule.Type, "local_port", rule.LocalPort)
	return nil
}

// StopForward はフォワーディングセッションを停止する。
func (m *forwardManager) StopForward(ruleName string) error {
	m.mu.Lock()
	session := m.stopForwardLocked(ruleName)
	m.mu.Unlock()

	if session != nil {
		m.emit(core.ForwardEvent{
			Type:     core.ForwardEventStopped,
			RuleName: ruleName,
			Session:  session,
		})
		slog.Info("forward stopped", "rule", ruleName)
	}
	return nil
}

// StopAllForwards は全フォワーディングセッションを停止する。
func (m *forwardManager) StopAllForwards() error {
	m.mu.RLock()
	names := make([]string, 0, len(m.active))
	for name := range m.active {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if err := m.StopForward(name); err != nil {
			return err
		}
	}
	return nil
}

// stopForwardLocked はロック保持中にフォワーディングセッションを停止する。
// 呼び出し元が m.mu.Lock() を保持していること。
// 停止したセッション情報を返す（アクティブでない場合は nil）。
func (m *forwardManager) stopForwardLocked(ruleName string) *core.ForwardSession {
	af, exists := m.active[ruleName]
	if !exists {
		return nil
	}

	// 起動中プレースホルダーの場合はエントリを削除するのみ
	if af.starting {
		delete(m.active, ruleName)
		return nil
	}

	_ = af.listener.Close()
	af.cancel()
	af.session.Status = core.Stopped
	af.session.BytesSent = af.sent.Load()
	af.session.BytesReceived = af.received.Load()
	session := af.session
	delete(m.active, ruleName)
	return &session
}

// Close は全フォワーディングを停止し、サブスクライバーチャネルを閉じる。
func (m *forwardManager) Close() {
	_ = m.StopAllForwards()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
}
