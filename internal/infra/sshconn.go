package infra

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ousiassllc/moleport/internal/core"
)

// SSHConnection は SSH 接続とポートフォワーディングの低レベル操作を提供する。
type SSHConnection interface {
	// Dial はホストへ SSH 接続を確立する。
	// cb が nil の場合、SSH エージェントと鍵ファイルのみで認証する。
	// cb が非 nil の場合、パスワード・パスフレーズ・keyboard-interactive 認証も試行する。
	Dial(host core.SSHHost, cb core.CredentialCallback) (*ssh.Client, error)

	// Close は接続を閉じる。
	Close() error

	// LocalForward はローカルポートフォワーディングのリスナーを開始する。
	LocalForward(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)

	// RemoteForward はリモートポートフォワーディングのリスナーを開始する。
	RemoteForward(ctx context.Context, remotePort int, localAddr string) (net.Listener, error)

	// DynamicForward はダイナミックフォワーディング（SOCKS）のリスナーを開始する。
	DynamicForward(ctx context.Context, localPort int) (net.Listener, error)

	// IsAlive は接続が生きているかを返す。
	IsAlive() bool

	// KeepAlive はキープアライブのティッカーループを実行する。
	KeepAlive(ctx context.Context, interval time.Duration)
}

type sshConnection struct {
	mu          sync.Mutex
	client      *ssh.Client
	agentCloser io.Closer
}

// NewSSHConnection は SSHConnection の実装を返す。
func NewSSHConnection() SSHConnection {
	return &sshConnection{}
}

func (c *sshConnection) Dial(host core.SSHHost, cb core.CredentialCallback) (*ssh.Client, error) {
	authMethods, agentCloser := buildAuthMethods(host, cb)
	if len(authMethods) == 0 {
		if agentCloser != nil {
			agentCloser.Close()
		}
		return nil, fmt.Errorf("no authentication methods available for host %s", host.Name)
	}

	closeAgent := func() {
		if agentCloser != nil {
			agentCloser.Close()
		}
	}

	hostKeyCallback, err := buildHostKeyCallback()
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
	timeout := 10 * time.Second

	// TCP 接続（タイムアウト付き）
	tcpConn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		closeAgent()
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	// TCP + SSH ハンドシェイク全体にデッドラインを設定
	if err := tcpConn.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = tcpConn.Close()
		closeAgent()
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// SSH ハンドシェイク（デッドラインが適用される）
	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, config)
	if err != nil {
		_ = tcpConn.Close()
		closeAgent()
		return nil, fmt.Errorf("failed to establish SSH connection to %s: %w", addr, err)
	}

	// ハンドシェイク完了後、デッドラインをクリア
	if err := tcpConn.SetDeadline(time.Time{}); err != nil {
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

func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
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

	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
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

	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
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
