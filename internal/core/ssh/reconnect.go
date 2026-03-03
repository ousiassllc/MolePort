package ssh

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math"
	"math/big"
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

// backoffWithJitter は指数バックオフにジッター（0-10%）を加えた遅延を計算する。
func backoffWithJitter(current, maxDelay time.Duration) time.Duration {
	base := time.Duration(math.Min(float64(current)*2, float64(maxDelay)))
	// 0-10% のジッターを crypto/rand で生成
	maxJitter := int64(float64(base) * 0.1)
	if maxJitter <= 0 {
		return base
	}
	n, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
	if err != nil {
		return base
	}
	return base + time.Duration(n.Int64())
}

// disconnectState は handleDisconnect の初期状態を保持する。
type disconnectState struct {
	host         core.SSHHost
	reconnectCfg core.ReconnectConfig
}

// handleDisconnect は切断検出時の自動再接続を処理する。
func (m *sshManager) handleDisconnect(hostName string) {
	ds, ok := m.cleanupDisconnected(hostName)
	if !ok {
		return
	}

	m.emit(core.SSHEvent{Type: core.SSHEventDisconnected, HostName: hostName})

	if !ds.reconnectCfg.Enabled {
		return
	}

	m.reconnectLoop(hostName, ds)
}

// cleanupDisconnected は切断されたホストの状態をクリーンアップし、再接続に必要な情報を返す。
// ホストが存在しない場合や既にキャンセル済みの場合は false を返す。
func (m *sshManager) cleanupDisconnected(hostName string) (disconnectState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hc, exists := m.conns[hostName]
	if !exists {
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
	reconnectCtx, reconnectCancel := context.WithCancel(context.Background())
	defer reconnectCancel()

	m.registerReconnectCancel(hostName, reconnectCancel)

	m.emit(core.SSHEvent{Type: core.SSHEventReconnecting, HostName: hostName})
	m.setHostState(hostName, core.Reconnecting)

	delay := ds.reconnectCfg.InitialDelay.Duration
	maxDelay := ds.reconnectCfg.MaxDelay.Duration

	for attempt := 0; attempt < ds.reconnectCfg.MaxRetries; attempt++ {
		slog.Info("attempting reconnect", "host", hostName, "attempt", attempt+1, "delay", delay)

		select {
		case <-reconnectCtx.Done():
			return
		case <-time.After(delay):
		}

		if m.isClosed() {
			return
		}

		if m.tryReconnect(hostName, ds.host) {
			return
		}

		slog.Warn("reconnect failed", "host", hostName, "attempt", attempt+1)
		delay = backoffWithJitter(delay, maxDelay)
	}

	// 再接続失敗
	m.mu.Lock()
	delete(m.reconnectCancels, hostName)
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.ConnectionError
	}
	m.mu.Unlock()

	m.emit(core.SSHEvent{Type: core.SSHEventError, HostName: hostName,
		Error: fmt.Errorf("reconnect failed after %d attempts", ds.reconnectCfg.MaxRetries)})
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

// tryReconnect は1回の再接続を試行し、成功時は true を返す。
func (m *sshManager) tryReconnect(hostName string, host core.SSHHost) bool {
	conn := m.connFactory()
	client, err := conn.Dial(host, nil)
	if err != nil {
		slog.Warn("reconnect dial failed", "host", hostName, "error", err)
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	hc := &hostConnection{
		conn:   conn,
		client: client,
		ctx:    ctx,
		cancel: cancel,
		state:  core.Connected,
	}

	m.mu.Lock()
	m.conns[hostName] = hc
	delete(m.reconnectCancels, hostName)
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.Connected
	}
	m.mu.Unlock()

	m.emit(core.SSHEvent{Type: core.SSHEventConnected, HostName: hostName})
	slog.Info("SSH reconnected", "host", hostName)

	go func() {
		conn.KeepAlive(ctx, m.keepAliveInterval())
		select {
		case <-ctx.Done():
			return
		default:
			m.handleDisconnect(hostName)
		}
	}()

	return true
}
