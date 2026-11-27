package forward

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

var errManagerClosed = errors.New("forward manager is closed")

// StartForward はフォワーディングセッションを開始する。
// cb が非 nil の場合、SSH 接続にクレデンシャルコールバックを使用する。
func (m *forwardManager) StartForward(ruleName string, cb core.CredentialCallback) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errManagerClosed
	}

	rule, exists := m.rules[ruleName]
	if !exists {
		m.mu.Unlock()
		return &core.NotFoundError{Resource: "rule", Name: ruleName}
	}

	if existing, active := m.active[ruleName]; active {
		// 再接続失敗・復元失敗で Error になった転送は再開できる。旧リスナー・ctx を
		// イベントなし（Stopped を出さない）で破棄してから開始処理を続ける。
		if existing.starting || existing.session.Status != core.SessionError {
			m.mu.Unlock()
			return &core.AlreadyActiveError{Name: ruleName}
		}
		m.stopForwardLocked(ruleName)
	}

	// 開始中プレースホルダー。Stop / Delete / Close はこのエントリを active から外すため、
	// 開始処理側は m.active[ruleName] が自分のプレースホルダーのままかで中断要求を検知する。
	placeholder := &activeForward{starting: true}
	m.active[ruleName] = placeholder
	m.mu.Unlock()

	cleanup := func() {
		m.mu.Lock()
		if m.active[ruleName] == placeholder {
			delete(m.active, ruleName)
		}
		m.mu.Unlock()
	}

	// stillStarting は開始処理が中断されていなければ nil を返す。
	stillStarting := func() error {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.active[ruleName] != placeholder || m.closed {
			return fmt.Errorf("forward start cancelled: %s", ruleName)
		}
		return nil
	}

	if !m.sshManager.IsConnected(rule.Host) {
		if err := m.sshManager.ConnectWithCallback(rule.Host, cb); err != nil {
			cleanup()
			return fmt.Errorf("failed to connect to host %s: %w", rule.Host, err)
		}
	}

	if err := stillStarting(); err != nil {
		cleanup()
		return err
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

	ctx, cancel := context.WithCancel(m.ctx)

	listener, err := openListener(ctx, sshConn, rule)
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

	// 開始処理中に Stop / Delete / Close された場合は、確立したリスナーを破棄する。
	m.mu.Lock()
	if m.active[ruleName] != placeholder || m.closed {
		if m.active[ruleName] == placeholder {
			delete(m.active, ruleName)
		}
		m.mu.Unlock()
		_ = listener.Close()
		cancel()
		return fmt.Errorf("forward start cancelled: %s", ruleName)
	}
	m.active[ruleName] = af
	m.mu.Unlock()

	go m.acceptLoop(af, rule, sshClient)

	m.events.Emit(core.ForwardEvent{
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
		m.events.Emit(core.ForwardEvent{
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

	// 起動中プレースホルダーの場合はエントリを削除するのみ。
	// 開始処理側が active の不一致を検知して中断する。
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
	// 先に closed を立て、停止中に新たな開始が確立されないようにする。
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()

	_ = m.StopAllForwards()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.events.CloseSubscribers()
}
