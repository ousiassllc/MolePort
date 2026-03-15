package infra

import (
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

// Dial は指定ホストへ SSH 接続を確立する。
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
			if err := agentCloser.Close(); err != nil {
				slog.Debug("failed to close SSH agent connection", "error", err)
			}
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

	// ProxyJump が設定されていても現在は未対応のため警告を出力し、代替手段を案内
	if len(host.ProxyJump) > 0 {
		slog.Warn("ProxyJump is not supported; use ProxyCommand instead (e.g. ProxyCommand ssh -W %h:%p <jumphost>)",
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

// Close は SSH 接続とエージェント接続を閉じる。
func (c *sshConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.agentCloser != nil {
		if err := c.agentCloser.Close(); err != nil {
			slog.Debug("failed to close SSH agent connection", "error", err)
		}
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
