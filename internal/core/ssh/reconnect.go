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

// handleDisconnect は切断検出時の自動再接続を処理する。
func (m *sshManager) handleDisconnect(hostName string) {
	m.mu.Lock()
	hc, exists := m.conns[hostName]
	if !exists {
		m.mu.Unlock()
		return
	}

	// 既にキャンセルされている場合（明示的な Disconnect）は再接続しない
	select {
	case <-hc.ctx.Done():
		m.mu.Unlock()
		return
	default:
	}

	// 接続をクリーンアップ
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
	m.mu.Unlock()

	m.emit(core.SSHEvent{Type: core.SSHEventDisconnected, HostName: hostName})

	if !reconnectCfg.Enabled {
		return
	}

	// ホストごとの再接続コンテキストを作成
	reconnectCtx, reconnectCancel := context.WithCancel(context.Background())
	defer reconnectCancel()

	m.mu.Lock()
	// 既存の再接続をキャンセル
	if oldCancel, exists := m.reconnectCancels[hostName]; exists {
		oldCancel()
	}
	m.reconnectCancels[hostName] = reconnectCancel
	m.mu.Unlock()

	m.emit(core.SSHEvent{Type: core.SSHEventReconnecting, HostName: hostName})

	m.mu.Lock()
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.Reconnecting
	}
	m.mu.Unlock()

	delay := reconnectCfg.InitialDelay.Duration
	maxDelay := reconnectCfg.MaxDelay.Duration

	for attempt := 0; attempt < reconnectCfg.MaxRetries; attempt++ {
		slog.Info("attempting reconnect", "host", hostName, "attempt", attempt+1, "delay", delay)

		// ホストごとのキャンセルを考慮して待機
		select {
		case <-reconnectCtx.Done():
			return
		case <-time.After(delay):
		}

		// マネージャーが閉じられていないか確認
		m.mu.RLock()
		if m.closed {
			m.mu.RUnlock()
			return
		}
		m.mu.RUnlock()

		conn := m.connFactory()
		client, err := conn.Dial(host, nil)
		if err != nil {
			slog.Warn("reconnect failed", "host", hostName, "attempt", attempt+1, "error", err)
			// 指数バックオフ + ジッター
			delay = backoffWithJitter(delay, maxDelay)
			continue
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

		return
	}

	// 再接続失敗
	m.mu.Lock()
	delete(m.reconnectCancels, hostName)
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.ConnectionError
	}
	m.mu.Unlock()

	m.emit(core.SSHEvent{Type: core.SSHEventError, HostName: hostName,
		Error: fmt.Errorf("reconnect failed after %d attempts", reconnectCfg.MaxRetries)})
}
