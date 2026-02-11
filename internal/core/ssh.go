package core

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// defaultKeepAliveInterval は SSH 接続の KeepAlive 送信間隔。
const defaultKeepAliveInterval = 30 * time.Second

// SSHConfigParser は SSH config ファイルを解析しホスト定義を抽出する。
// infra.SSHConfigParser と同じインターフェースで、import cycle を回避するために core で定義する。
type SSHConfigParser interface {
	Parse(configPath string) ([]SSHHost, error)
}

// SSHConnection は SSH 接続とポートフォワーディングの低レベル操作を提供する。
// infra.SSHConnection と同じインターフェースで、import cycle を回避するために core で定義する。
type SSHConnection interface {
	Dial(host SSHHost) (*ssh.Client, error)
	Close() error
	LocalForward(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)
	RemoteForward(ctx context.Context, remotePort int, localAddr string) (net.Listener, error)
	DynamicForward(ctx context.Context, localPort int) (net.Listener, error)
	IsAlive() bool
	KeepAlive(ctx context.Context, interval time.Duration)
}

// SSHManager は SSH 接続のライフサイクルを管理する。
type SSHManager interface {
	LoadHosts() ([]SSHHost, error)
	ReloadHosts() ([]SSHHost, error)
	GetHost(name string) (*SSHHost, error)
	Connect(hostName string) error
	Disconnect(hostName string) error
	IsConnected(hostName string) bool
	GetConnection(hostName string) (*ssh.Client, error)
	GetSSHConnection(hostName string) (SSHConnection, error)
	Subscribe() <-chan SSHEvent
	Close()
}

// hostConnection は個々のホストへの接続状態を保持する。
type hostConnection struct {
	conn   SSHConnection
	client *ssh.Client
	ctx    context.Context
	cancel context.CancelFunc
	state  ConnectionState
}

type sshManager struct {
	mu           sync.RWMutex
	parser       SSHConfigParser
	connFactory  func() SSHConnection
	configPath   string
	reconnectCfg ReconnectConfig

	hosts            []SSHHost
	hostsMap         map[string]int
	conns            map[string]*hostConnection
	reconnectCancels map[string]context.CancelFunc // ホストごとの再接続キャンセル関数
	subscribers      []chan SSHEvent

	closed bool
}

// NewSSHManager は SSHManager の実装を返す。
func NewSSHManager(
	parser SSHConfigParser,
	connFactory func() SSHConnection,
	configPath string,
	reconnectCfg ReconnectConfig,
) SSHManager {
	return &sshManager{
		parser:           parser,
		connFactory:      connFactory,
		configPath:       configPath,
		reconnectCfg:     reconnectCfg,
		hostsMap:         make(map[string]int),
		conns:            make(map[string]*hostConnection),
		reconnectCancels: make(map[string]context.CancelFunc),
	}
}

// LoadHosts は SSH config を解析してホスト一覧を構築する。
func (m *sshManager) LoadHosts() ([]SSHHost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hosts, err := m.parser.Parse(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	m.hosts = hosts
	m.hostsMap = make(map[string]int, len(hosts))
	for i, h := range hosts {
		m.hostsMap[h.Name] = i
	}

	return m.copyHosts(), nil
}

// ReloadHosts は SSH config を再解析し、既存の接続状態を保持する。
func (m *sshManager) ReloadHosts() ([]SSHHost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hosts, err := m.parser.Parse(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	// 既存の接続状態を保持
	oldStates := make(map[string]ConnectionState)
	oldForwards := make(map[string]int)
	for _, h := range m.hosts {
		oldStates[h.Name] = h.State
		oldForwards[h.Name] = h.ActiveForwardCount
	}

	for i := range hosts {
		if state, ok := oldStates[hosts[i].Name]; ok {
			hosts[i].State = state
			hosts[i].ActiveForwardCount = oldForwards[hosts[i].Name]
		}
	}

	m.hosts = hosts
	m.hostsMap = make(map[string]int, len(hosts))
	for i, h := range hosts {
		m.hostsMap[h.Name] = i
	}

	return m.copyHosts(), nil
}

// GetHost は名前でホストを取得する。
func (m *sshManager) GetHost(name string) (*SSHHost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, ok := m.hostsMap[name]
	if !ok {
		return nil, fmt.Errorf("host %q not found", name)
	}
	h := m.hosts[idx]
	return &h, nil
}

// Connect はホストへ SSH 接続を確立する。
func (m *sshManager) Connect(hostName string) error {
	m.mu.Lock()
	idx, ok := m.hostsMap[hostName]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("host %q not found", hostName)
	}

	// 既に接続中または接続処理中の場合は何もしない
	if hc, exists := m.conns[hostName]; exists && (hc.state == Connected || hc.state == Connecting) {
		m.mu.Unlock()
		return nil
	}

	// 接続処理中として登録（同一ホストへの並行 Connect を防ぐ）
	hcConnecting := &hostConnection{state: Connecting}
	m.conns[hostName] = hcConnecting

	host := m.hosts[idx]
	m.hosts[idx].State = Connecting
	m.mu.Unlock()

	conn := m.connFactory()
	client, err := conn.Dial(host)
	if err != nil {
		m.mu.Lock()
		// Connecting プレースホルダーを削除
		if current, exists := m.conns[hostName]; exists && current == hcConnecting {
			delete(m.conns, hostName)
		}
		if i, ok := m.hostsMap[hostName]; ok {
			m.hosts[i].State = ConnectionError
		}
		m.mu.Unlock()
		m.emit(SSHEvent{Type: SSHEventError, HostName: hostName, Error: err})
		return fmt.Errorf("failed to connect to %s: %w", hostName, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	hc := &hostConnection{
		conn:   conn,
		client: client,
		ctx:    ctx,
		cancel: cancel,
		state:  Connected,
	}

	m.mu.Lock()
	m.conns[hostName] = hc
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = Connected
	}
	m.mu.Unlock()

	// KeepAlive goroutine
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

	m.emit(SSHEvent{Type: SSHEventConnected, HostName: hostName})
	slog.Info("SSH connected", "host", hostName)
	return nil
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
	hc.conn.Close()
	hc.state = Disconnected

	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = Disconnected
	}

	reconnectCfg := m.reconnectCfg
	var host SSHHost
	if idx, ok := m.hostsMap[hostName]; ok {
		host = m.hosts[idx]
	}
	delete(m.conns, hostName)
	m.mu.Unlock()

	m.emit(SSHEvent{Type: SSHEventDisconnected, HostName: hostName})

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

	m.emit(SSHEvent{Type: SSHEventReconnecting, HostName: hostName})

	m.mu.Lock()
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = Reconnecting
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
		client, err := conn.Dial(host)
		if err != nil {
			slog.Warn("reconnect failed", "host", hostName, "attempt", attempt+1, "error", err)
			// 指数バックオフ
			delay = time.Duration(math.Min(float64(delay)*2, float64(maxDelay)))
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		hc := &hostConnection{
			conn:   conn,
			client: client,
			ctx:    ctx,
			cancel: cancel,
			state:  Connected,
		}

		m.mu.Lock()
		m.conns[hostName] = hc
		delete(m.reconnectCancels, hostName)
		if i, ok := m.hostsMap[hostName]; ok {
			m.hosts[i].State = Connected
		}
		m.mu.Unlock()

		go func() {
			conn.KeepAlive(ctx, defaultKeepAliveInterval)
			select {
			case <-ctx.Done():
				return
			default:
				m.handleDisconnect(hostName)
			}
		}()

		m.emit(SSHEvent{Type: SSHEventConnected, HostName: hostName})
		slog.Info("SSH reconnected", "host", hostName)
		return
	}

	// 再接続失敗
	m.mu.Lock()
	delete(m.reconnectCancels, hostName)
	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = ConnectionError
	}
	m.mu.Unlock()

	m.emit(SSHEvent{Type: SSHEventError, HostName: hostName,
		Error: fmt.Errorf("reconnect failed after %d attempts", reconnectCfg.MaxRetries)})
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
	hc.state = Disconnected
	delete(m.conns, hostName)

	if i, ok := m.hostsMap[hostName]; ok {
		m.hosts[i].State = Disconnected
	}
	m.mu.Unlock()

	m.emit(SSHEvent{Type: SSHEventDisconnected, HostName: hostName})
	slog.Info("SSH disconnected", "host", hostName)
	return nil
}

// IsConnected はホストが接続中かを返す。
func (m *sshManager) IsConnected(hostName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hc, exists := m.conns[hostName]
	return exists && hc.state == Connected
}

// GetConnection は接続済みホストの *ssh.Client を返す。
func (m *sshManager) GetConnection(hostName string) (*ssh.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hc, exists := m.conns[hostName]
	if !exists || hc.state != Connected {
		return nil, fmt.Errorf("host %q is not connected", hostName)
	}
	return hc.client, nil
}

// GetSSHConnection は接続済みホストの SSHConnection を返す。
func (m *sshManager) GetSSHConnection(hostName string) (SSHConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hc, exists := m.conns[hostName]
	if !exists || hc.state != Connected {
		return nil, fmt.Errorf("host %q is not connected", hostName)
	}
	return hc.conn, nil
}

// Subscribe はイベントチャネルを返す。
func (m *sshManager) Subscribe() <-chan SSHEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan SSHEvent, 16)
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
		hc.state = Disconnected
		if i, ok := m.hostsMap[name]; ok {
			m.hosts[i].State = Disconnected
		}
	}
	m.conns = make(map[string]*hostConnection)

	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
}

// emit はイベントを全サブスクライバーに非ブロッキングで送信する。
func (m *sshManager) emit(event SSHEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// copyHosts はホスト一覧のコピーを返す。mu.Lock の中で呼ぶこと。
func (m *sshManager) copyHosts() []SSHHost {
	result := make([]SSHHost, len(m.hosts))
	copy(result, m.hosts)
	return result
}
