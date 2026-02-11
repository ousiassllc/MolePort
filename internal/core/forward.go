package core

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// SOCKS5 プロトコル定数
const (
	socks5Version      = 0x05
	socks5NoAuth       = 0x00
	socks5NoAcceptable = 0xFF
	socks5CmdConnect   = 0x01
	socks5AddrIPv4     = 0x01
	socks5AddrDomain   = 0x03
	socks5AddrIPv6     = 0x04

	// SOCKS5 reply codes
	socks5ReplySuccess              = 0x00
	socks5ReplyCommandNotSupported  = 0x07
	socks5ReplyAddrTypeNotSupported = 0x08
	socks5ReplyConnectionRefused    = 0x05
)

// ForwardManager はポートフォワーディングルールとセッションを管理する。
type ForwardManager interface {
	AddRule(rule ForwardRule) error
	DeleteRule(name string) error
	GetRules() []ForwardRule
	GetRulesByHost(hostName string) []ForwardRule
	StartForward(ruleName string) error
	StopForward(ruleName string) error
	StopAllForwards() error
	GetSession(ruleName string) (*ForwardSession, error)
	GetAllSessions() []ForwardSession
	Subscribe() <-chan ForwardEvent
	Close()
}

// activeForward は実行中のフォワーディングセッションを保持する。
type activeForward struct {
	session  ForwardSession
	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	sent     atomic.Int64
	received atomic.Int64
}

type forwardManager struct {
	mu          sync.RWMutex
	sshManager  SSHManager
	rules       map[string]ForwardRule
	ruleOrder   []string // 追加順序を保持
	active      map[string]*activeForward
	subscribers []chan ForwardEvent
	closed      bool
	nextID      int
}

// NewForwardManager は ForwardManager の実装を返す。
func NewForwardManager(sshManager SSHManager) ForwardManager {
	return &forwardManager{
		sshManager: sshManager,
		rules:      make(map[string]ForwardRule),
		active:     make(map[string]*activeForward),
	}
}

// AddRule はフォワーディングルールを追加する。
func (m *forwardManager) AddRule(rule ForwardRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 名前が空の場合は自動生成
	if rule.Name == "" {
		m.nextID++
		rule.Name = fmt.Sprintf("forward-%d", m.nextID)
	}

	// 名前の一意性チェック
	if _, exists := m.rules[rule.Name]; exists {
		return fmt.Errorf("rule %q already exists", rule.Name)
	}

	// バリデーション
	if rule.Host == "" {
		return fmt.Errorf("host is required")
	}

	if rule.LocalPort < 1 || rule.LocalPort > 65535 {
		return fmt.Errorf("local_port must be between 1 and 65535, got %d", rule.LocalPort)
	}

	if rule.Type == Local || rule.Type == Remote {
		if rule.RemotePort < 1 || rule.RemotePort > 65535 {
			return fmt.Errorf("remote_port must be between 1 and 65535, got %d", rule.RemotePort)
		}
		if rule.RemoteHost == "" {
			rule.RemoteHost = "localhost"
		}
	}

	m.rules[rule.Name] = rule
	m.ruleOrder = append(m.ruleOrder, rule.Name)
	return nil
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
		m.emit(ForwardEvent{
			Type:     ForwardEventStopped,
			RuleName: name,
			Session:  session,
		})
		slog.Info("forward stopped", "rule", name)
	}
	return nil
}

// stopForwardLocked はロック保持中にフォワーディングセッションを停止する。
// 呼び出し元が m.mu.Lock() を保持していること。
// 停止したセッション情報を返す（アクティブでない場合は nil）。
func (m *forwardManager) stopForwardLocked(ruleName string) *ForwardSession {
	af, exists := m.active[ruleName]
	if !exists {
		return nil
	}

	af.listener.Close()
	af.cancel()
	af.session.Status = Stopped
	af.session.BytesSent = af.sent.Load()
	af.session.BytesReceived = af.received.Load()
	session := af.session
	delete(m.active, ruleName)
	return &session
}

// GetRules は全ルールを追加順に返す。
func (m *forwardManager) GetRules() []ForwardRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]ForwardRule, 0, len(m.ruleOrder))
	for _, name := range m.ruleOrder {
		if rule, ok := m.rules[name]; ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetRulesByHost はホスト名でフィルタしたルール一覧を返す。
func (m *forwardManager) GetRulesByHost(hostName string) []ForwardRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rules []ForwardRule
	for _, name := range m.ruleOrder {
		if rule, ok := m.rules[name]; ok && rule.Host == hostName {
			rules = append(rules, rule)
		}
	}
	return rules
}

// StartForward はフォワーディングセッションを開始する。
func (m *forwardManager) StartForward(ruleName string) error {
	m.mu.Lock()
	rule, exists := m.rules[ruleName]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("rule %q not found", ruleName)
	}

	if _, active := m.active[ruleName]; active {
		m.mu.Unlock()
		return fmt.Errorf("forward %q is already active", ruleName)
	}
	m.mu.Unlock()

	// SSH 接続を確認（必要に応じて接続）
	if !m.sshManager.IsConnected(rule.Host) {
		if err := m.sshManager.Connect(rule.Host); err != nil {
			return fmt.Errorf("failed to connect to host %s: %w", rule.Host, err)
		}
	}

	sshConn, err := m.sshManager.GetSSHConnection(rule.Host)
	if err != nil {
		return fmt.Errorf("failed to get SSH connection: %w", err)
	}

	sshClient, err := m.sshManager.GetConnection(rule.Host)
	if err != nil {
		return fmt.Errorf("failed to get SSH client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var listener net.Listener
	switch rule.Type {
	case Local:
		remoteAddr := fmt.Sprintf("%s:%d", rule.RemoteHost, rule.RemotePort)
		listener, err = sshConn.LocalForward(ctx, rule.LocalPort, remoteAddr)
	case Remote:
		localAddr := fmt.Sprintf("127.0.0.1:%d", rule.LocalPort)
		listener, err = sshConn.RemoteForward(ctx, rule.RemotePort, localAddr)
	case Dynamic:
		listener, err = sshConn.DynamicForward(ctx, rule.LocalPort)
	default:
		cancel()
		return fmt.Errorf("unsupported forward type: %v", rule.Type)
	}

	if err != nil {
		cancel()
		return fmt.Errorf("failed to create listener: %w", err)
	}

	af := &activeForward{
		session: ForwardSession{
			ID:          fmt.Sprintf("%s-%d", ruleName, time.Now().UnixNano()),
			Rule:        rule,
			Status:      Active,
			ConnectedAt: time.Now(),
		},
		listener: listener,
		ctx:      ctx,
		cancel:   cancel,
	}

	m.mu.Lock()
	m.active[ruleName] = af
	m.mu.Unlock()

	// accept ループを開始
	go m.acceptLoop(af, rule, sshClient)

	m.emit(ForwardEvent{
		Type:     ForwardEventStarted,
		RuleName: ruleName,
		Session:  &af.session,
	})

	slog.Info("forward started", "rule", ruleName, "type", rule.Type, "local_port", rule.LocalPort)
	return nil
}

// acceptLoop はリスナーで接続を受け付け、ブリッジを作成する。
func (m *forwardManager) acceptLoop(af *activeForward, rule ForwardRule, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) {
	for {
		conn, err := af.listener.Accept()
		if err != nil {
			select {
			case <-af.ctx.Done():
				return
			default:
				slog.Warn("accept error", "rule", rule.Name, "error", err)
				return
			}
		}

		go m.bridge(af, rule, conn, sshClient)
	}
}

// dialRemote はルールの種類に応じてリモート接続を確立する。
func (m *forwardManager) dialRemote(rule ForwardRule, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) (net.Conn, error) {
	switch rule.Type {
	case Local:
		remoteAddr := fmt.Sprintf("%s:%d", rule.RemoteHost, rule.RemotePort)
		return sshClient.Dial("tcp", remoteAddr)
	case Remote:
		localAddr := fmt.Sprintf("127.0.0.1:%d", rule.LocalPort)
		return net.Dial("tcp", localAddr)
	default:
		return nil, fmt.Errorf("unsupported forward type for bridge: %v", rule.Type)
	}
}

// bridge は受け付けた接続とリモート/ローカルの間でデータを転送する。
func (m *forwardManager) bridge(af *activeForward, rule ForwardRule, conn net.Conn, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) {
	defer conn.Close()

	if rule.Type == Dynamic {
		m.handleSOCKS5(af, conn, sshClient)
		return
	}

	remote, err := m.dialRemote(rule, sshClient)
	if err != nil {
		slog.Warn("bridge dial failed", "rule", rule.Name, "error", err)
		return
	}
	defer remote.Close()

	m.copyBidirectional(af, conn, remote)
}

// socks5Negotiate は SOCKS5 の挨拶・認証ネゴシエーションを処理する。
func socks5Negotiate(conn net.Conn) error {
	// Client greeting: VER + NMETHODS (2 bytes)
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}

	if header[0] != socks5Version {
		return fmt.Errorf("unsupported SOCKS version: %d", header[0])
	}

	nmethods := int(header[1])
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	// クライアントが No Authentication をサポートしているか確認
	noAuthSupported := false
	for _, method := range methods {
		if method == socks5NoAuth {
			noAuthSupported = true
			break
		}
	}
	if !noAuthSupported {
		// No acceptable methods
		_, _ = conn.Write([]byte{socks5Version, socks5NoAcceptable})
		return fmt.Errorf("no acceptable auth methods")
	}

	// No authentication
	if _, err := conn.Write([]byte{socks5Version, socks5NoAuth}); err != nil {
		return err
	}
	return nil
}

// socks5ParseRequest は SOCKS5 の CONNECT リクエストを解析し、接続先アドレスを返す。
func socks5ParseRequest(conn net.Conn) (string, error) {
	// SOCKS5 request: VER, CMD, RSV, ATYP (4 bytes)
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return "", err
	}

	if reqHeader[0] != socks5Version || reqHeader[1] != socks5CmdConnect {
		// Command not supported
		_, _ = conn.Write([]byte{socks5Version, socks5ReplyCommandNotSupported, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return "", fmt.Errorf("unsupported SOCKS5 command: %d", reqHeader[1])
	}

	switch reqHeader[3] { // Address type
	case socks5AddrIPv4:
		addrPort := make([]byte, 4+2)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		ip := net.IP(addrPort[:4])
		port := binary.BigEndian.Uint16(addrPort[4:6])
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), nil
	case socks5AddrDomain:
		domainLenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, domainLenBuf); err != nil {
			return "", err
		}
		domainLen := int(domainLenBuf[0])
		domainPort := make([]byte, domainLen+2)
		if _, err := io.ReadFull(conn, domainPort); err != nil {
			return "", err
		}
		domain := string(domainPort[:domainLen])
		port := binary.BigEndian.Uint16(domainPort[domainLen : domainLen+2])
		return net.JoinHostPort(domain, strconv.Itoa(int(port))), nil
	case socks5AddrIPv6:
		addrPort := make([]byte, 16+2)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		ip := net.IP(addrPort[:16])
		port := binary.BigEndian.Uint16(addrPort[16:18])
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), nil
	default:
		// Address type not supported
		_, _ = conn.Write([]byte{socks5Version, socks5ReplyAddrTypeNotSupported, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return "", fmt.Errorf("unsupported address type: %d", reqHeader[3])
	}
}

// handleSOCKS5 は最小限の SOCKS5 プロトコルを処理する（認証なし、CONNECT のみ）。
func (m *forwardManager) handleSOCKS5(af *activeForward, conn net.Conn, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) {
	if err := socks5Negotiate(conn); err != nil {
		slog.Debug("socks5 negotiate failed", "rule", af.session.Rule.Name, "error", err)
		return
	}

	targetAddr, err := socks5ParseRequest(conn)
	if err != nil {
		slog.Debug("socks5 parse request failed", "rule", af.session.Rule.Name, "error", err)
		return
	}

	remote, err := sshClient.Dial("tcp", targetAddr)
	if err != nil {
		// Connection refused
		_, _ = conn.Write([]byte{socks5Version, socks5ReplyConnectionRefused, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remote.Close()

	// Success response
	if _, err := conn.Write([]byte{socks5Version, socks5ReplySuccess, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	m.copyBidirectional(af, conn, remote)
}

// copyBidirectional は二つの接続間でデータを双方向にコピーする。
func (m *forwardManager) copyBidirectional(af *activeForward, a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		n, err := io.Copy(b, a)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
			slog.Debug("copy error", "rule", af.session.Rule.Name, "error", err)
		}
		af.sent.Add(n)
	}()

	go func() {
		defer wg.Done()
		n, err := io.Copy(a, b)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
			slog.Debug("copy error", "rule", af.session.Rule.Name, "error", err)
		}
		af.received.Add(n)
	}()

	wg.Wait()
}

// StopForward はフォワーディングセッションを停止する。
func (m *forwardManager) StopForward(ruleName string) error {
	m.mu.Lock()
	session := m.stopForwardLocked(ruleName)
	m.mu.Unlock()

	if session != nil {
		m.emit(ForwardEvent{
			Type:     ForwardEventStopped,
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

// GetSession はルール名からセッション情報を返す。
func (m *forwardManager) GetSession(ruleName string) (*ForwardSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if af, exists := m.active[ruleName]; exists {
		session := af.session
		session.BytesSent = af.sent.Load()
		session.BytesReceived = af.received.Load()
		return &session, nil
	}

	rule, exists := m.rules[ruleName]
	if !exists {
		return nil, fmt.Errorf("rule %q not found", ruleName)
	}

	return &ForwardSession{
		Rule:   rule,
		Status: Stopped,
	}, nil
}

// GetAllSessions は全ルールのセッション情報を返す。
func (m *forwardManager) GetAllSessions() []ForwardSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]ForwardSession, 0, len(m.ruleOrder))
	for _, name := range m.ruleOrder {
		rule, ok := m.rules[name]
		if !ok {
			continue
		}

		if af, active := m.active[name]; active {
			session := af.session
			session.BytesSent = af.sent.Load()
			session.BytesReceived = af.received.Load()
			sessions = append(sessions, session)
		} else {
			sessions = append(sessions, ForwardSession{
				Rule:   rule,
				Status: Stopped,
			})
		}
	}
	return sessions
}

// Subscribe はイベントチャネルを返す。
func (m *forwardManager) Subscribe() <-chan ForwardEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan ForwardEvent, 16)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

// Close は全フォワーディングを停止し、サブスクライバーチャネルを閉じる。
func (m *forwardManager) Close() {
	m.StopAllForwards()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
}

// emit はイベントを全サブスクライバーに非ブロッキングで送信する。
func (m *forwardManager) emit(event ForwardEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
