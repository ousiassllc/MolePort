package infra

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

// testSSHServer はテスト用の SSH サーバー。
type testSSHServer struct {
	Addr    string
	Host    string
	Port    int
	HostKey ssh.Signer
	ln      net.Listener
}

type testSSHServerOption func(*ssh.ServerConfig)

// withPublicKeyAuth は公開鍵認証を有効にする。
func withPublicKeyAuth(authorizedKey ssh.PublicKey) testSSHServerOption {
	return func(cfg *ssh.ServerConfig) {
		cfg.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), authorizedKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized key")
		}
	}
}

// withPasswordAuth はパスワード認証を有効にする。
func withPasswordAuth(password string) testSSHServerOption {
	return func(cfg *ssh.ServerConfig) {
		cfg.PasswordCallback = func(_ ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("invalid password")
		}
	}
}

// newTestSSHServer はテスト用 SSH サーバーを起動して返す。
// opts が空の場合は NoClientAuth で動作する。
func newTestSSHServer(t *testing.T, opts ...testSSHServerOption) *testSSHServer {
	t.Helper()
	return startTestSSHServer(t, handleTestSSHConn, opts...)
}

// newTestSSHServerWithTCPIPForward は tcpip-forward をサポートするテスト用 SSH サーバー。
func newTestSSHServerWithTCPIPForward(t *testing.T, opts ...testSSHServerOption) *testSSHServer {
	t.Helper()
	return startTestSSHServer(t, handleTestSSHConnWithTCPIPForward, opts...)
}

type connHandler func(net.Conn, *ssh.ServerConfig)

func startTestSSHServer(t *testing.T, handler connHandler, opts ...testSSHServerOption) *testSSHServer {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	cfg := &ssh.ServerConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.PublicKeyCallback == nil && cfg.PasswordCallback == nil && cfg.KeyboardInteractiveCallback == nil {
		cfg.NoClientAuth = true
	}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	addr := ln.Addr().(*net.TCPAddr)
	s := &testSSHServer{
		Addr:    ln.Addr().String(),
		Host:    addr.IP.String(),
		Port:    addr.Port,
		HostKey: hostSigner,
		ln:      ln,
	}

	go func() {
		for {
			conn, err := s.ln.Accept()
			if err != nil {
				return
			}
			go handler(conn, cfg)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })

	return s
}

func handleTestSSHConn(netConn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, cfg)
	if err != nil {
		_ = netConn.Close()
		return
	}
	defer func() { _ = sshConn.Close() }()

	go func() {
		for req := range reqs {
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
		}
	}()

	for newCh := range chans {
		_ = newCh.Reject(ssh.Prohibited, "not supported")
	}
}

func handleTestSSHConnWithTCPIPForward(netConn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, cfg)
	if err != nil {
		_ = netConn.Close()
		return
	}
	defer func() { _ = sshConn.Close() }()

	go func() {
		for req := range reqs {
			switch req.Type {
			case "tcpip-forward":
				handleTCPIPForward(sshConn, req)
			default:
				if req.WantReply {
					_ = req.Reply(true, nil)
				}
			}
		}
	}()

	for newCh := range chans {
		_ = newCh.Reject(ssh.Prohibited, "not supported")
	}
}

type tcpipForwardRequest struct {
	BindAddr string
	BindPort uint32
}

type tcpipForwardResponse struct {
	BoundPort uint32
}

func handleTCPIPForward(sshConn *ssh.ServerConn, req *ssh.Request) {
	var payload tcpipForwardRequest
	if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
		if req.WantReply {
			_ = req.Reply(false, nil)
		}
		return
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", payload.BindAddr, payload.BindPort))
	if err != nil {
		if req.WantReply {
			_ = req.Reply(false, nil)
		}
		return
	}

	boundPort := uint32(ln.Addr().(*net.TCPAddr).Port) //nolint:gosec // テスト用、ポート番号は uint16 範囲内
	if req.WantReply {
		_ = req.Reply(true, ssh.Marshal(tcpipForwardResponse{BoundPort: boundPort}))
	}

	go func() {
		_ = sshConn.Wait() //nolint:errcheck
		_ = ln.Close()
	}()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
}

func testSSHHost(s *testSSHServer) core.SSHHost {
	return core.SSHHost{
		Name:                  "test-server",
		HostName:              s.Host,
		Port:                  s.Port,
		User:                  "testuser",
		StrictHostKeyChecking: "no",
	}
}
