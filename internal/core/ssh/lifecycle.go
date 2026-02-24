package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

// isAuthFailure はエラーが認証失敗を示すかどうかを判定する。
func isAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no authentication methods available") ||
		strings.Contains(msg, "no supported methods remain")
}

// Connect はホストへ SSH 接続を確立する。
func (m *sshManager) Connect(hostName string) error {
	return m.connectInternal(hostName, nil)
}

// ConnectWithCallback はホストへ SSH 接続を確立する（クレデンシャルコールバック付き）。
func (m *sshManager) ConnectWithCallback(hostName string, cb core.CredentialCallback) error {
	return m.connectInternal(hostName, cb)
}

// GetPendingAuthHosts は pending_auth 状態のホスト名一覧を返す。
func (m *sshManager) GetPendingAuthHosts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var hosts []string
	for _, h := range m.hosts {
		if h.State == core.PendingAuth {
			hosts = append(hosts, h.Name)
		}
	}
	return hosts
}

// connectInternal は Connect と ConnectWithCallback の共通実装。
func (m *sshManager) connectInternal(hostName string, cb core.CredentialCallback) error {
	m.mu.Lock()
	idx, ok := m.hostsMap[hostName]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("host %q not found", hostName)
	}

	// 既に接続中または接続処理中の場合は何もしない
	if hc, exists := m.conns[hostName]; exists && (hc.state == core.Connected || hc.state == core.Connecting) {
		m.mu.Unlock()
		return nil
	}

	// 接続処理中として登録（同一ホストへの並行 Connect を防ぐ）
	hcConnecting := &hostConnection{state: core.Connecting}
	m.conns[hostName] = hcConnecting

	host := m.hosts[idx]
	m.hosts[idx].State = core.Connecting
	m.mu.Unlock()

	conn := m.connFactory()
	client, err := conn.Dial(host, cb)
	if err != nil {
		m.mu.Lock()
		// Connecting プレースホルダーを削除
		if current, exists := m.conns[hostName]; exists && current == hcConnecting {
			delete(m.conns, hostName)
		}

		// 認証失敗の場合は PendingAuth 状態にする（コールバックなしの場合のみ）
		if cb == nil && isAuthFailure(err) {
			if i, ok := m.hostsMap[hostName]; ok {
				m.hosts[i].State = core.PendingAuth
			}
			m.mu.Unlock()
			m.emit(core.SSHEvent{Type: core.SSHEventPendingAuth, HostName: hostName})
			return fmt.Errorf("authentication required for %s: %w", hostName, err)
		}

		if i, ok := m.hostsMap[hostName]; ok {
			m.hosts[i].State = core.ConnectionError
		}
		m.mu.Unlock()
		m.emit(core.SSHEvent{Type: core.SSHEventError, HostName: hostName, Error: err})
		return fmt.Errorf("failed to connect to %s: %w", hostName, err)
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
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.Connected
	}
	m.mu.Unlock()

	m.emit(core.SSHEvent{Type: core.SSHEventConnected, HostName: hostName})
	slog.Info("SSH connected", "host", hostName)

	// KeepAlive goroutine
	// Connected イベント emit 後に起動して、イベント順序を保証する
	go func() {
		conn.KeepAlive(ctx, defaultKeepAliveInterval)
		// KeepAlive が終了した場合（コンテキストキャンセル以外）、切断を検出
		select {
		case <-ctx.Done():
			return
		default:
			m.handleDisconnect(hostName)
		}
	}()

	return nil
}

// Disconnect はホストとの接続を切断する。
func (m *sshManager) Disconnect(hostName string) error {
	m.mu.Lock()
	// 進行中の再接続をキャンセル
	if reconnectCancel, exists := m.reconnectCancels[hostName]; exists {
		reconnectCancel()
		delete(m.reconnectCancels, hostName)
	}

	hc, exists := m.conns[hostName]
	if !exists {
		m.mu.Unlock()
		return nil
	}

	if hc.cancel != nil {
		hc.cancel()
	}
	if hc.conn != nil {
		hc.conn.Close()
	}
	hc.state = core.Disconnected
	delete(m.conns, hostName)

	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = core.Disconnected
	}
	m.mu.Unlock()

	m.emit(core.SSHEvent{Type: core.SSHEventDisconnected, HostName: hostName})
	slog.Info("SSH disconnected", "host", hostName)
	return nil
}

// IsConnected はホストが接続中かを返す。
func (m *sshManager) IsConnected(hostName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hc, exists := m.conns[hostName]
	return exists && hc.state == core.Connected
}

// GetConnection は接続済みホストの *cryptossh.Client を返す。
func (m *sshManager) GetConnection(hostName string) (*cryptossh.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hc, exists := m.conns[hostName]
	if !exists || hc.state != core.Connected {
		return nil, fmt.Errorf("host %q is not connected", hostName)
	}
	return hc.client, nil
}

// GetSSHConnection は接続済みホストの SSHConnection を返す。
func (m *sshManager) GetSSHConnection(hostName string) (core.SSHConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hc, exists := m.conns[hostName]
	if !exists || hc.state != core.Connected {
		return nil, fmt.Errorf("host %q is not connected", hostName)
	}
	return hc.conn, nil
}

// Subscribe はイベントチャネルを返す。
func (m *sshManager) Subscribe() <-chan core.SSHEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan core.SSHEvent, 16)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

// Close は全接続を切断し、サブスクライバーチャネルを閉じる。
func (m *sshManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true

	// 進行中の再接続をすべてキャンセル
	for name, cancel := range m.reconnectCancels {
		cancel()
		delete(m.reconnectCancels, name)
	}

	for name, hc := range m.conns {
		if hc.cancel != nil {
			hc.cancel()
		}
		if hc.conn != nil {
			hc.conn.Close()
		}
		hc.state = core.Disconnected
		if i, ok := m.hostsMap[name]; ok {
			m.hosts[i].State = core.Disconnected
		}
	}
	m.conns = make(map[string]*hostConnection)

	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
}
