package forward

import (
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// halfCloseConn は net.Conn をラップし、CloseWrite の呼び出しを記録する。
type halfCloseConn struct {
	net.Conn
	closeWriteCalled atomic.Bool
}

func (c *halfCloseConn) CloseWrite() error {
	c.closeWriteCalled.Store(true)
	return nil
}

// plainConn は CloseWrite を持たない net.Conn ラッパー。
// Close の呼び出しを記録する。
type plainConn struct {
	net.Conn
	mu          sync.Mutex
	closeCalled int
}

func (c *plainConn) Close() error {
	c.mu.Lock()
	c.closeCalled++
	c.mu.Unlock()
	return c.Conn.Close()
}

// doSOCKS5Connect はSOCKS5グリーティング・リクエストを送信し、接続先アドレスを返す。
func doSOCKS5Connect(t *testing.T, request []byte) string {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	dialedAddr := make(chan string, 1)
	dialer := &mockSOCKS5Dialer{dialF: func(_, addr string) (net.Conn, error) {
		dialedAddr <- addr
		rc, _ := net.Pipe()
		return rc, nil
	}}
	fm := NewForwardManager(newMockSSHManager()).(*forwardManager)
	go fm.handleSOCKS5(&activeForward{}, serverConn, dialer)

	_, _ = clientConn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, resp); err != nil {
		t.Fatalf("read greeting response: %v", err)
	}
	if !bytes.Equal(resp, []byte{0x05, 0x00}) {
		t.Fatalf("unexpected greeting response: %v", resp)
	}
	_, _ = clientConn.Write(request)
	successResp := make([]byte, 10)
	if _, err := io.ReadFull(clientConn, successResp); err != nil {
		t.Fatalf("read success response: %v", err)
	}
	if successResp[0] != 0x05 || successResp[1] != 0x00 {
		t.Fatalf("unexpected success response: %v", successResp)
	}
	select {
	case addr := <-dialedAddr:
		return addr
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dial")
		return ""
	}
}

func TestHandleSOCKS5_ConnectVariants(t *testing.T) {
	// Domain type: example.com:80
	domainReq := []byte{0x05, 0x01, 0x00, 0x03, byte(len("example.com"))} //nolint:gosec // domain length is always < 256
	domainReq = append(domainReq, []byte("example.com")...)
	domainReq = append(domainReq, 0x00, 0x50)
	// IPv4 type: 192.168.1.1:8080
	ipv4Req := []byte{0x05, 0x01, 0x00, 0x01, 192, 168, 1, 1, 0x1F, 0x90}
	// IPv6 type: [::1]:443
	ipv6Req := []byte{0x05, 0x01, 0x00, 0x04}
	ipv6Req = append(ipv6Req, net.ParseIP("::1").To16()...)
	ipv6Req = append(ipv6Req, 0x01, 0xBB)

	tests := []struct {
		name    string
		request []byte
		want    string
	}{
		{"Domain", domainReq, "example.com:80"},
		{"IPv4", ipv4Req, "192.168.1.1:8080"},
		{"IPv6", ipv6Req, net.JoinHostPort("::1", "443")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := doSOCKS5Connect(t, tt.request)
			if got != tt.want {
				t.Errorf("dialed addr = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleSOCKS5_NoAuthMethodRejected(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	fm := NewForwardManager(newMockSSHManager()).(*forwardManager)
	done := make(chan struct{})
	go func() {
		defer close(done)
		fm.handleSOCKS5(&activeForward{}, serverConn, &mockSOCKS5Dialer{})
	}()

	_, _ = clientConn.Write([]byte{0x05, 0x01, 0x02}) // username/password only
	resp := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, resp); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !bytes.Equal(resp, []byte{0x05, 0xFF}) {
		t.Errorf("expected no acceptable methods (0xFF), got %v", resp)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleSOCKS5 did not return after rejection")
	}
}

func TestHandleSOCKS5_FragmentedWrites(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	dialedAddr := make(chan string, 1)
	dialer := &mockSOCKS5Dialer{dialF: func(_, addr string) (net.Conn, error) {
		dialedAddr <- addr
		rc, _ := net.Pipe()
		return rc, nil
	}}
	fm := NewForwardManager(newMockSSHManager()).(*forwardManager)
	af := &activeForward{session: core.ForwardSession{Rule: core.ForwardRule{Name: "test"}}}
	go fm.handleSOCKS5(af, serverConn, dialer)

	for _, b := range []byte{0x05, 0x01, 0x00} {
		_, _ = clientConn.Write([]byte{b})
		time.Sleep(time.Millisecond)
	}
	resp := make([]byte, 2)
	_, _ = io.ReadFull(clientConn, resp)

	domain := "a.b"
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(domain))} //nolint:gosec // domain length is always < 256
	req = append(req, []byte(domain)...)
	req = append(req, 0x00, 0x50)
	for _, b := range req {
		_, _ = clientConn.Write([]byte{b})
		time.Sleep(time.Millisecond)
	}
	successResp := make([]byte, 10)
	_, _ = io.ReadFull(clientConn, successResp)

	select {
	case addr := <-dialedAddr:
		if addr != "a.b:80" {
			t.Errorf("dialed addr = %q, want %q", addr, "a.b:80")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

func TestCopyBidirectional_HalfClose(t *testing.T) {
	aClient, aServer := net.Pipe()
	bClient, bServer := net.Pipe()
	defer func() { _ = aClient.Close() }()
	defer func() { _ = bClient.Close() }()
	hcA := &halfCloseConn{Conn: aServer}
	hcB := &halfCloseConn{Conn: bServer}
	fm := NewForwardManager(newMockSSHManager()).(*forwardManager)
	af := &activeForward{session: core.ForwardSession{Rule: core.ForwardRule{Name: "hc-test"}}}
	done := make(chan struct{})
	go func() { defer close(done); fm.copyBidirectional(af, hcA, hcB) }()

	_, _ = aClient.Write([]byte("hello"))
	_ = aClient.Close()
	buf := make([]byte, 5)
	if _, err := io.ReadFull(bClient, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = bClient.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if !hcB.closeWriteCalled.Load() {
		t.Error("CloseWrite not called on b")
	}
	if !hcA.closeWriteCalled.Load() {
		t.Error("CloseWrite not called on a")
	}
}

func TestCopyBidirectional_FallbackClose(t *testing.T) {
	aClient, aServer := net.Pipe()
	bClient, bServer := net.Pipe()
	defer func() { _ = aClient.Close() }()
	defer func() { _ = bClient.Close() }()
	pcA := &plainConn{Conn: aServer}
	pcB := &plainConn{Conn: bServer}
	fm := NewForwardManager(newMockSSHManager()).(*forwardManager)
	af := &activeForward{session: core.ForwardSession{Rule: core.ForwardRule{Name: "fb-test"}}}
	done := make(chan struct{})
	go func() { defer close(done); fm.copyBidirectional(af, pcA, pcB) }()

	_ = aClient.Close()
	_ = bClient.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	pcA.mu.Lock()
	ac := pcA.closeCalled
	pcA.mu.Unlock()
	pcB.mu.Lock()
	bc := pcB.closeCalled
	pcB.mu.Unlock()
	if ac < 1 {
		t.Error("Close not called on a")
	}
	if bc < 1 {
		t.Error("Close not called on b")
	}
}
