package forward

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/ousiassllc/moleport/internal/core"
)

// MarkReconnecting は当該ホストのアクティブセッションを SessionReconnecting 状態にする。
func (m *forwardManager) MarkReconnecting(hostName string) {
	var events []core.ForwardEvent

	m.mu.Lock()
	for _, af := range m.active {
		if af.starting {
			continue
		}
		if af.session.Rule.Host == hostName && af.session.Status == core.Active {
			_ = af.listener.Close()
			af.cancel()
			af.session.Status = core.SessionReconnecting
			af.session.BytesSent = af.sent.Load()
			af.session.BytesReceived = af.received.Load()
			session := af.session
			events = append(events, core.ForwardEvent{
				Type:     core.ForwardEventReconnecting,
				RuleName: af.session.Rule.Name,
				Session:  &session,
			})
		}
	}
	m.mu.Unlock()

	for _, evt := range events {
		m.emit(evt)
	}
}

// RestoreForwards は SSH 再接続後に SessionReconnecting 状態の全フォワードを復元する。
func (m *forwardManager) RestoreForwards(hostName string) []core.ForwardRestoreResult {
	// SessionReconnecting 状態のフォワードを収集
	m.mu.RLock()
	var targets []*activeForward
	for _, af := range m.active {
		if af.starting {
			continue
		}
		if af.session.Rule.Host == hostName && af.session.Status == core.SessionReconnecting {
			targets = append(targets, af)
		}
	}
	m.mu.RUnlock()

	if len(targets) == 0 {
		return nil
	}

	// 新しい SSH 接続を取得
	sshConn, sshConnErr := m.sshManager.GetSSHConnection(hostName)
	sshClient, sshClientErr := m.sshManager.GetConnection(hostName)

	results := make([]core.ForwardRestoreResult, 0, len(targets))
	for _, af := range targets {
		result := m.restoreSingleForward(af, sshConn, sshConnErr, sshClient, sshClientErr)
		results = append(results, result)
	}
	return results
}

// restoreSingleForward は単一のフォワードを復元する。
func (m *forwardManager) restoreSingleForward(
	af *activeForward,
	sshConn core.SSHConnection,
	sshConnErr error,
	sshClient interface {
		Dial(n, addr string) (net.Conn, error)
	},
	sshClientErr error,
) core.ForwardRestoreResult {
	rule := af.session.Rule

	if sshConnErr != nil {
		m.setForwardError(af, sshConnErr.Error())
		return core.ForwardRestoreResult{RuleName: rule.Name, OK: false, Error: sshConnErr.Error()}
	}
	if sshClientErr != nil {
		m.setForwardError(af, sshClientErr.Error())
		return core.ForwardRestoreResult{RuleName: rule.Name, OK: false, Error: sshClientErr.Error()}
	}

	ctx, cancel := context.WithCancel(m.ctx)

	listener, err := openListener(ctx, sshConn, rule)

	if err != nil {
		cancel()
		m.setForwardError(af, err.Error())
		return core.ForwardRestoreResult{RuleName: rule.Name, OK: false, Error: err.Error()}
	}

	// 成功: 新しい activeForward を作成して置き換え（旧 acceptLoop とのデータレースを回避）
	m.mu.Lock()
	// 復元対象がまだ active マップに存在し SessionReconnecting であることを再確認
	if current, exists := m.active[rule.Name]; !exists || current != af || af.session.Status != core.SessionReconnecting {
		m.mu.Unlock()
		cancel()
		_ = listener.Close()
		return core.ForwardRestoreResult{RuleName: rule.Name, OK: false, Error: "forward was stopped during restoration"}
	}

	newAF := &activeForward{
		session: core.ForwardSession{
			ID:             af.session.ID,
			Rule:           rule,
			Status:         core.Active,
			ConnectedAt:    af.session.ConnectedAt,
			BytesSent:      af.sent.Load(),
			BytesReceived:  af.received.Load(),
			ReconnectCount: af.session.ReconnectCount + 1,
		},
		listener: listener,
		ctx:      ctx,
		cancel:   cancel,
	}
	m.active[rule.Name] = newAF
	session := newAF.session
	m.mu.Unlock()

	go m.acceptLoop(newAF, rule, sshClient)

	m.emit(core.ForwardEvent{
		Type:     core.ForwardEventRestored,
		RuleName: rule.Name,
		Session:  &session,
	})

	slog.Info("forward restored", "rule", rule.Name, "reconnect_count", session.ReconnectCount)
	return core.ForwardRestoreResult{RuleName: rule.Name, OK: true}
}

// setForwardError はフォワードを SessionError 状態にし、ForwardEventError を発行する。
func (m *forwardManager) setForwardError(af *activeForward, errMsg string) {
	m.mu.Lock()
	af.session.Status = core.SessionError
	af.session.LastError = errMsg
	session := af.session
	m.mu.Unlock()

	m.emit(core.ForwardEvent{
		Type:     core.ForwardEventError,
		RuleName: session.Rule.Name,
		Session:  &session,
		Error:    errors.New(errMsg),
	})
}

// FailReconnecting は再接続失敗時に SessionReconnecting 状態のフォワードを Error 状態にする。
func (m *forwardManager) FailReconnecting(hostName string) {
	var events []core.ForwardEvent

	m.mu.Lock()
	for _, af := range m.active {
		if af.starting {
			continue
		}
		if af.session.Rule.Host == hostName && af.session.Status == core.SessionReconnecting {
			af.session.Status = core.SessionError
			af.session.LastError = "reconnection failed"
			session := af.session
			events = append(events, core.ForwardEvent{
				Type:     core.ForwardEventError,
				RuleName: session.Rule.Name,
				Session:  &session,
				Error:    errors.New("reconnection failed"),
			})
		}
	}
	m.mu.Unlock()

	for _, evt := range events {
		m.emit(evt)
	}
}
