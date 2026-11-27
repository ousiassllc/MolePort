package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// resolveReconnectConfig はグローバル設定にホスト別オーバーライドをマージして返す。
func resolveReconnectConfig(global core.ReconnectConfig, override *core.ReconnectOverride) core.ReconnectConfig {
	if override == nil {
		return global
	}
	result := global
	if override.Enabled != nil {
		result.Enabled = *override.Enabled
	}
	if override.MaxRetries != nil {
		result.MaxRetries = *override.MaxRetries
	}
	if override.InitialDelay != nil {
		result.InitialDelay = *override.InitialDelay
	}
	if override.MaxDelay != nil {
		result.MaxDelay = *override.MaxDelay
	}
	return result
}

// disconnectState は handleDisconnect の初期状態を保持する。
type disconnectState struct {
	host         core.SSHHost
	reconnectCfg core.ReconnectConfig
}

// handleDisconnect は切断検出時の自動再接続を処理する。
func (m *sshManager) handleDisconnect(hostName string, hc *hostConnection) {
	ds, ok := m.cleanupDisconnected(hostName, hc)
	if !ok {
		return
	}

	m.events.Emit(core.SSHEvent{Type: core.SSHEventDisconnected, HostName: hostName})

	if !ds.reconnectCfg.Enabled {
		return
	}

	m.reconnectLoop(hostName, ds)
}

// cleanupDisconnected は切断されたホストの状態をクリーンアップし、再接続に必要な情報を返す。
// 現在の接続が hc と異なる場合（置き換え済みの古い接続の切断検出）や、ホストが存在しない場合、
// 既にキャンセル済みの場合は false を返す。
func (m *sshManager) cleanupDisconnected(hostName string, hc *hostConnection) (disconnectState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if current, exists := m.conns[hostName]; !exists || current != hc {
		return disconnectState{}, false
	}

	// 既にキャンセルされている場合（明示的な Disconnect）は再接続しない
	select {
	case <-hc.ctx.Done():
		return disconnectState{}, false
	default:
	}

	hc.cancel()
	_ = hc.conn.Close()
	hc.state = core.Disconnected

	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.Disconnected
	}

	reconnectCfg := m.reconnectCfg
	if hostCfg, ok := m.hostConfigs[hostName]; ok {
		reconnectCfg = resolveReconnectConfig(reconnectCfg, hostCfg.Reconnect)
	}
	var host core.SSHHost
	if idx, ok := m.hostsMap[hostName]; ok {
		host = m.hosts[idx]
	}
	delete(m.conns, hostName)

	return disconnectState{host: host, reconnectCfg: reconnectCfg}, true
}

// reconnectLoop は指数バックオフ付きで再接続を試行する。
func (m *sshManager) reconnectLoop(hostName string, ds disconnectState) {
	reconnectCtx, reconnectCancel := context.WithCancel(m.ctx)
	defer reconnectCancel()

	m.registerReconnectCancel(hostName, reconnectCancel)

	m.events.Emit(core.SSHEvent{Type: core.SSHEventReconnecting, HostName: hostName})
	m.setHostState(hostName, core.Reconnecting)

	// base はジッター加算前の遅延。ジッターを累積させないため、倍化は base に対して行う。
	maxDelay := ds.reconnectCfg.MaxDelay.Duration
	base := initialBackoff(ds.reconnectCfg.InitialDelay.Duration, maxDelay)

	// MaxRetries == 0 は無制限（Disconnect / Close まで再試行を続ける）
	maxRetries := ds.reconnectCfg.MaxRetries
	for attempt := 0; maxRetries == 0 || attempt < maxRetries; attempt++ {
		wait := withJitter(base, maxDelay)
		slog.Info("attempting reconnect", "host", hostName, "attempt", attempt+1, "delay", wait)

		select {
		case <-reconnectCtx.Done():
			return
		case <-time.After(wait):
		}

		if m.isClosed() {
			return
		}

		switch m.tryReconnect(reconnectCtx, hostName, ds.host) {
		case reconnectSucceeded, reconnectCancelled:
			return
		case reconnectAuthFailed:
			m.pendAuthAfterReconnect(reconnectCtx, hostName)
			return
		case reconnectFailed:
		}

		slog.Warn("reconnect failed", "host", hostName, "attempt", attempt+1)
		base = nextBackoff(base, maxDelay)
	}

	// 再接続失敗
	m.mu.Lock()
	if reconnectCtx.Err() != nil {
		// 最後の試行中に Disconnect / Close された場合はエラーにしない
		m.mu.Unlock()
		return
	}
	delete(m.reconnectCancels, hostName)
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.ConnectionError
	}
	m.mu.Unlock()

	m.events.Emit(core.SSHEvent{Type: core.SSHEventError, HostName: hostName,
		Error: fmt.Errorf("reconnect failed after %d attempts", maxRetries)})
}

// pendAuthAfterReconnect は再接続中の認証失敗でホストを PendingAuth にし、リトライを打ち切る。
// 再接続ではクレデンシャルを提供できず、同じ認証を繰り返しても成功しない（サーバー側の
// ロックアウトも招く）ため、ユーザーの手動 connect を待つ。
func (m *sshManager) pendAuthAfterReconnect(reconnectCtx context.Context, hostName string) {
	m.mu.Lock()
	if m.closed || reconnectCtx.Err() != nil {
		// 試行中に Disconnect / Close された場合は、その結果を上書きしない
		m.mu.Unlock()
		return
	}
	delete(m.reconnectCancels, hostName)
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.PendingAuth
	}
	m.mu.Unlock()

	slog.Warn("reconnect stopped: authentication failed, manual connect required", "host", hostName)
	m.events.Emit(core.SSHEvent{Type: core.SSHEventPendingAuth, HostName: hostName})
}

// registerReconnectCancel は再接続キャンセル関数を登録し、既存のものがあればキャンセルする。
func (m *sshManager) registerReconnectCancel(hostName string, cancel context.CancelFunc) {
	m.mu.Lock()
	if oldCancel, exists := m.reconnectCancels[hostName]; exists {
		oldCancel()
	}
	m.reconnectCancels[hostName] = cancel
	m.mu.Unlock()
}

// setHostState はホストの接続状態を更新する。
func (m *sshManager) setHostState(hostName string, state core.ConnectionState) {
	m.mu.Lock()
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = state
	}
	m.mu.Unlock()
}

// isClosed はマネージャーが閉じられているかを返す。
func (m *sshManager) isClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// reconnectResult は 1 回の再接続試行の結果を表す。
type reconnectResult int

const (
	// reconnectSucceeded は再接続に成功し、接続が登録されたことを表す。
	reconnectSucceeded reconnectResult = iota
	// reconnectFailed は試行が失敗し、バックオフ後に再試行できることを表す。
	reconnectFailed
	// reconnectAuthFailed は認証失敗でリトライしても成功しないことを表す。
	reconnectAuthFailed
	// reconnectCancelled は試行中に Disconnect / Close されたため、再接続を打ち切ることを表す。
	reconnectCancelled
)

// tryReconnect は1回の再接続を試行する。
// Dial 中に ctx がキャンセルされた（Disconnect）、またはマネージャーが閉じられた場合は、
// 確立した接続を破棄して reconnectCancelled を返す。
func (m *sshManager) tryReconnect(reconnectCtx context.Context, hostName string, host core.SSHHost) reconnectResult {
	conn := m.connFactory()
	client, err := conn.Dial(host, nil)
	if err != nil {
		slog.Warn("reconnect dial failed", "host", hostName, "error", err)
		if isAuthFailure(err) {
			return reconnectAuthFailed
		}
		return reconnectFailed
	}

	ctx, cancel := context.WithCancel(m.ctx) //nolint:gosec // cancel は hc.cancel に保持され Disconnect 時に呼ばれる
	hc := &hostConnection{
		conn:   conn,
		client: client,
		ctx:    ctx,
		cancel: cancel,
		state:  core.Connected,
	}

	m.mu.Lock()
	// Disconnect は reconnectCancels 経由で reconnectCtx をキャンセルする（m.mu 下）。
	// 待機中に別経路で接続が確立された場合も、その接続を優先して確立した接続を破棄する。
	if _, occupied := m.conns[hostName]; m.closed || occupied || reconnectCtx.Err() != nil {
		m.mu.Unlock()
		cancel()
		_ = conn.Close()
		return reconnectCancelled
	}
	m.conns[hostName] = hc
	delete(m.reconnectCancels, hostName)
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.Connected
	}
	m.mu.Unlock()

	m.events.Emit(core.SSHEvent{Type: core.SSHEventConnected, HostName: hostName})
	slog.Info("SSH reconnected", "host", hostName)

	go func() {
		conn.KeepAlive(ctx, m.keepAliveInterval())
		select {
		case <-ctx.Done():
			return
		default:
			m.handleDisconnect(hostName, hc)
		}
	}()

	return reconnectSucceeded
}
