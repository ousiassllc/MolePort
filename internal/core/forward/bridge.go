package forward

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/socks5"
)

// halfCloser は TCP half-close をサポートする接続を表す。
// net.TCPConn はこのインターフェースを満たすが、SSH チャネル経由の接続は
// 満たさない場合がある。
type halfCloser interface {
	CloseWrite() error
}

// bufPool は io.CopyBuffer で使用するバッファの再利用プール。
// バッファサイズは io.Copy のデフォルト (32KB) と同じ。
var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

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
		localAddr := net.JoinHostPort(core.LocalhostAddr, fmt.Sprintf("%d", rule.LocalPort))
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
	defer func() { _ = remote.Close() }()

	m.copyBidirectional(af, conn, remote)
}

// handleSOCKS5 は最小限の SOCKS5 プロトコルを処理する（認証なし、CONNECT のみ）。
func (m *forwardManager) handleSOCKS5(af *activeForward, conn net.Conn, sshClient interface {
	Dial(n, addr string) (net.Conn, error)
}) {
	if err := socks5.Negotiate(conn); err != nil {
		slog.Debug("socks5 negotiate failed", "rule", af.session.Rule.Name, "error", err)
		return
	}

	targetAddr, err := socks5.ParseRequest(conn)
	if err != nil {
		slog.Debug("socks5 parse request failed", "rule", af.session.Rule.Name, "error", err)
		return
	}

	remote, err := sshClient.Dial("tcp", targetAddr)
	if err != nil {
		// Connection refused
		_, _ = conn.Write([]byte{socks5.Version, socks5.ReplyConnectionRefused, 0x00, socks5.AddrIPv4, 0, 0, 0, 0, 0, 0})
		return
	}
	defer func() { _ = remote.Close() }()

	// Success response
	if _, err := conn.Write([]byte{socks5.Version, socks5.ReplySuccess, 0x00, socks5.AddrIPv4, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	m.copyBidirectional(af, conn, remote)
}

// copyBidirectional は二つの接続間でデータを双方向にコピーする。
// コピー完了後、half-close (CloseWrite) で EOF を相手側に伝播する。
func (m *forwardManager) copyBidirectional(af *activeForward, a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		bufp := bufPool.Get().(*[]byte) // safe: Pool.New always returns *[]byte
		defer bufPool.Put(bufp)
		n, err := io.CopyBuffer(b, a, *bufp)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
			slog.Debug("copy error", "rule", af.session.Rule.Name, "error", err)
		}
		af.sent.Add(n)
		closeWrite(b)
	}()

	go func() {
		defer wg.Done()
		bufp := bufPool.Get().(*[]byte) // safe: Pool.New always returns *[]byte
		defer bufPool.Put(bufp)
		n, err := io.CopyBuffer(a, b, *bufp)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
			slog.Debug("copy error", "rule", af.session.Rule.Name, "error", err)
		}
		af.received.Add(n)
		closeWrite(a)
	}()

	wg.Wait()
}

// closeWrite は接続の書き込み側を閉じる。
// halfCloser をサポートする場合は CloseWrite で half-close を行い、
// サポートしない場合は Close でフォールバックする。
func closeWrite(c net.Conn) {
	if hc, ok := c.(halfCloser); ok {
		_ = hc.CloseWrite()
	} else {
		_ = c.Close()
	}
}
