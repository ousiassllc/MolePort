package infra

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/infra/proxycommand"
)

const (
	defaultDialTimeout      = 10 * time.Second
	defaultHandshakeTimeout = 120 * time.Second
)

type sshConnection struct {
	mu          sync.Mutex
	client      *ssh.Client
	agentCloser io.Closer
}

// NewSSHConnection は core.SSHConnection の実装を返す。
func NewSSHConnection() core.SSHConnection {
	return &sshConnection{}
}

func (c *sshConnection) Dial(host core.SSHHost, cb core.CredentialCallback) (*ssh.Client, error) {
	authMethods, agentCloser := buildAuthMethods(host, cb)
	// authMethods が空でも早期リターンしない。
	// Go の crypto/ssh は常に "none" 認証を最初に試行するため、
	// Tailscale SSH のように none 認証で動作するサーバーへの接続が可能。
	if len(authMethods) == 0 {
		slog.Debug("no explicit auth methods configured, relying on none auth", "host", host.Name)
	}

	closeAgent := func() {
		if agentCloser != nil {
			agentCloser.Close()
		}
	}

	hostKeyCallback, err := buildHostKeyCallback(host.StrictHostKeyChecking)
	if err != nil {
		closeAgent()
		return nil, fmt.Errorf("failed to build host key callback: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            host.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	addr := net.JoinHostPort(host.HostName, fmt.Sprintf("%d", host.Port))
	dialTimeout := defaultDialTimeout

	// クレデンシャルコールバックがある場合、ハンドシェイク中にユーザー入力を待つため
	// デッドラインを長くする。
	handshakeTimeout := dialTimeout
	if cb != nil {
		handshakeTimeout = defaultHandshakeTimeout
	}

	// ProxyJump が設定されていても現在は未対応のため警告を出力
	if len(host.ProxyJump) > 0 {
		slog.Warn("ProxyJump is not supported, ignoring",
			"host", host.Name, "proxy_jump", host.ProxyJump)
	}

	// 接続（ProxyCommand の有無で分岐）
	// ProxyCommand が設定されている場合は ProxyJump より優先する（OpenSSH の挙動に準拠）。
	var conn net.Conn
	if host.ProxyCommand != "" {
		expandedCmd := proxycommand.ExpandCommand(host.ProxyCommand, host.HostName, host.Port, host.User)
		conn, err = proxycommand.Dial(expandedCmd)
		if err != nil {
			closeAgent()
			return nil, fmt.Errorf("failed to connect via ProxyCommand: %w", err)
		}
	} else {
		conn, err = net.DialTimeout("tcp", addr, dialTimeout)
		if err != nil {
			closeAgent()
			return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
		}
	}

	// TCP + SSH ハンドシェイク全体にデッドラインを設定
	if err := conn.SetDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		_ = conn.Close()
		closeAgent()
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// SSH ハンドシェイク（デッドラインが適用される）
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		closeAgent()
		return nil, fmt.Errorf("failed to establish SSH connection to %s: %w", addr, err)
	}

	// ハンドシェイク完了後、デッドラインをクリア
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = sshConn.Close()
		closeAgent()
		return nil, fmt.Errorf("failed to clear deadline: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	c.mu.Lock()
	c.client = client
	c.agentCloser = agentCloser
	c.mu.Unlock()

	return client, nil
}

func buildHostKeyCallback(strictHostKeyChecking string) (ssh.HostKeyCallback, error) {
	if strings.EqualFold(strictHostKeyChecking, "no") {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // SSH config の StrictHostKeyChecking=no を尊重
	}

	knownHostsPath := filepath.Join(homeDir(), ".ssh", "known_hosts")
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// known_hosts が存在しない場合は空ファイルを自動生成し、
			// 以降の接続でホストキーが記録されるようにする。
			slog.Warn("known_hosts file not found, creating empty file",
				"path", knownHostsPath)
			if mkErr := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); mkErr != nil {
				return nil, fmt.Errorf("failed to create .ssh directory: %w", mkErr)
			}
			if mkErr := os.WriteFile(knownHostsPath, nil, 0600); mkErr != nil {
				return nil, fmt.Errorf("failed to create known_hosts: %w", mkErr)
			}
			slog.Warn("known_hosts is empty; to trust host keys, run: ssh <host> manually",
				"path", knownHostsPath)
			// 空ファイルで再読込（全ホストキーを未知として扱う）
			callback, err = knownhosts.New(knownHostsPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load known_hosts after creation: %w", err)
			}
			return callback, nil
		}
		return nil, fmt.Errorf("failed to load known_hosts (%s): %w", knownHostsPath, err)
	}
	return callback, nil
}

func (c *sshConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.agentCloser != nil {
		c.agentCloser.Close()
		c.agentCloser = nil
	}

	if c.client == nil {
		return nil
	}
	err := c.client.Close()
	c.client = nil
	return err
}

func (c *sshConnection) getClient() *ssh.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client
}

// LocalForward はローカルポートフォワーディング用のリスナーを作成する。
// このメソッドはリスナーの作成のみを行い、accept ループやデータ転送は行わない。
// 呼び出し元（ForwardManager）が返されたリスナーで accept ループを実行し、
// Dial() で取得した ssh.Client を使ってリモートへのデータブリッジを行う。
func (c *sshConnection) LocalForward(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	addr := net.JoinHostPort(core.LocalhostAddr, fmt.Sprintf("%d", localPort))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	return listener, nil
}

// RemoteForward はリモートポートフォワーディング用のリスナーを作成する。
// このメソッドはリモート側リスナーの作成のみを行い、accept ループやデータ転送は行わない。
// 呼び出し元（ForwardManager）が返されたリスナーで accept ループを実行し、
// Dial() で取得した ssh.Client を使ってローカルへのデータブリッジを行う。
func (c *sshConnection) RemoteForward(ctx context.Context, remotePort int, localAddr string) (net.Listener, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	addr := fmt.Sprintf("0.0.0.0:%d", remotePort)
	listener, err := client.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen remotely on %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	return listener, nil
}

// DynamicForward はダイナミックフォワーディング（SOCKS プロキシ）用のリスナーを作成する。
// このメソッドはリスナーの作成のみを行い、SOCKS プロトコル処理やデータ転送は行わない。
// 呼び出し元（ForwardManager）が返されたリスナーで accept ループを実行し、
// Dial() で取得した ssh.Client を使って SOCKS プロキシのデータブリッジを行う。
func (c *sshConnection) DynamicForward(ctx context.Context, localPort int) (net.Listener, error) {
	client := c.getClient()
	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	addr := net.JoinHostPort(core.LocalhostAddr, fmt.Sprintf("%d", localPort))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	return listener, nil
}

func (c *sshConnection) IsAlive() bool {
	client := c.getClient()
	if client == nil {
		return false
	}

	_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

func (c *sshConnection) KeepAlive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.IsAlive() {
				slog.Warn("keepalive failed, connection may be lost")
				return
			}
		}
	}
}
