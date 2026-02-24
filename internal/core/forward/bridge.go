package forward

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/ousiassllc/moleport/internal/core"
)

// acceptLoop はリスナーで接続を受け付け、ブリッジを作成する。
func (m *forwardManager) acceptLoop(af *activeForward, rule core.ForwardRule, sshClient interface {
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
func (m *forwardManager) dialRemote(rule core.ForwardRule, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) (net.Conn, error) {
	switch rule.Type {
	case core.Local:
		remoteAddr := fmt.Sprintf("%s:%d", rule.RemoteHost, rule.RemotePort)
		return sshClient.Dial("tcp", remoteAddr)
	case core.Remote:
		localAddr := fmt.Sprintf("127.0.0.1:%d", rule.LocalPort)
		return net.Dial("tcp", localAddr)
	default:
		return nil, fmt.Errorf("unsupported forward type for bridge: %v", rule.Type)
	}
}

// bridge は受け付けた接続とリモート/ローカルの間でデータを転送する。
func (m *forwardManager) bridge(af *activeForward, rule core.ForwardRule, conn net.Conn, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) {
	defer func() { _ = conn.Close() }()

	if rule.Type == core.Dynamic {
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

// handleSOCKS5 は最小限の SOCKS5 プロトコルを処理する（認証なし、CONNECT のみ）。
func (m *forwardManager) handleSOCKS5(af *activeForward, conn net.Conn, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) {
	if err := core.Socks5Negotiate(conn); err != nil {
		slog.Debug("socks5 negotiate failed", "rule", af.session.Rule.Name, "error", err)
		return
	}

	targetAddr, err := core.Socks5ParseRequest(conn)
	if err != nil {
		slog.Debug("socks5 parse request failed", "rule", af.session.Rule.Name, "error", err)
		return
	}

	remote, err := sshClient.Dial("tcp", targetAddr)
	if err != nil {
		// Connection refused
		_, _ = conn.Write([]byte{core.Socks5Version, core.Socks5ReplyConnectionRefused, 0x00, core.Socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remote.Close()

	// Success response
	if _, err := conn.Write([]byte{core.Socks5Version, core.Socks5ReplySuccess, 0x00, core.Socks5AddrIPv4, 0, 0, 0, 0, 0, 0}); err != nil {
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
