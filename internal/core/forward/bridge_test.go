package forward

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestHandleSOCKS5_StagedReads(t *testing.T) {
	// SOCKS5 の段階的読み取りが正しく動作することを検証する。
	// クライアント側・サーバー側を net.Pipe で接続し、SOCKS5 ネゴシエーションを行う。
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			// remote 側もパイプで返す
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// Greeting: VER=5, NMETHODS=1, METHODS=[0x00]
	_, _ = clientConn.Write([]byte{0x05, 0x01, 0x00})

	// サーバーからの応答を読む
	resp := make([]byte, 2)
	n, err := io.ReadFull(clientConn, resp)
	if err != nil {
		t.Fatalf("read greeting response: %v", err)
	}
	if n < 2 || resp[0] != 0x05 || resp[1] != 0x00 {
		t.Fatalf("unexpected greeting response: %v", resp)
	}

	// Request: CONNECT to example.com:80 (domain type)
	domain := "example.com"
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(domain))} //nolint:gosec // domain length is always < 256
	req = append(req, []byte(domain)...)
	req = append(req, 0x00, 0x50) // port 80
	_, _ = clientConn.Write(req)

	// Success response
	successResp := make([]byte, 10)
	if _, err = io.ReadFull(clientConn, successResp); err != nil {
		t.Fatalf("read success response: %v", err)
	}
	if successResp[0] != 0x05 || successResp[1] != 0x00 {
		t.Fatalf("unexpected success response: %v", successResp)
	}

	// 正しいアドレスに接続されたことを確認
	select {
	case addr := <-dialedAddr:
		if addr != "example.com:80" {
			t.Errorf("dialed addr = %q, want %q", addr, "example.com:80")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

func TestHandleSOCKS5_IPv4(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// Greeting
	_, _ = clientConn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	_, _ = io.ReadFull(clientConn, resp)

	// Request: CONNECT to 192.168.1.1:8080 (IPv4)
	req := []byte{0x05, 0x01, 0x00, 0x01, 192, 168, 1, 1, 0x1F, 0x90} // port 8080
	_, _ = clientConn.Write(req)

	successResp := make([]byte, 10)
	_, _ = io.ReadFull(clientConn, successResp)

	select {
	case addr := <-dialedAddr:
		if addr != "192.168.1.1:8080" {
			t.Errorf("dialed addr = %q, want %q", addr, "192.168.1.1:8080")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

func TestHandleSOCKS5_IPv6(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// Greeting
	_, _ = clientConn.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	_, _ = io.ReadFull(clientConn, resp)

	// Request: CONNECT to [::1]:443 (IPv6)
	req := []byte{0x05, 0x01, 0x00, 0x04}
	ipv6 := net.ParseIP("::1").To16()
	req = append(req, ipv6...)
	req = append(req, 0x01, 0xBB) // port 443
	_, _ = clientConn.Write(req)

	successResp := make([]byte, 10)
	_, _ = io.ReadFull(clientConn, successResp)

	select {
	case addr := <-dialedAddr:
		expected := net.JoinHostPort("::1", "443")
		if addr != expected {
			t.Errorf("dialed addr = %q, want %q", addr, expected)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dial")
	}
}

func TestHandleSOCKS5_NoAuthMethodRejected(t *testing.T) {
	// クライアントが 0x00 (no auth) を含まないメソッドリストを送った場合、
	// サーバーは 0xFF (no acceptable methods) を返すことを検証する。
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{}

	mockDialer := &mockSOCKS5Dialer{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		fm.handleSOCKS5(af, serverConn, mockDialer)
	}()

	// Greeting: VER=5, NMETHODS=1, METHODS=[0x02] (username/password only)
	_, _ = clientConn.Write([]byte{0x05, 0x01, 0x02})

	resp := make([]byte, 2)
	n, err := io.ReadFull(clientConn, resp)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if n < 2 || resp[0] != 0x05 || resp[1] != 0xFF {
		t.Errorf("expected no acceptable methods (0xFF), got %v", resp)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleSOCKS5 did not return after rejection")
	}
}

func TestHandleSOCKS5_FragmentedWrites(t *testing.T) {
	// TCP ストリームで段階的に（1バイトずつ）送信しても正しく処理されることを確認する。
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	dialedAddr := make(chan string, 1)
	mockDialer := &mockSOCKS5Dialer{
		dialF: func(n, addr string) (net.Conn, error) {
			dialedAddr <- addr
			rc, _ := net.Pipe()
			return rc, nil
		},
	}

	sm := newMockSSHManager()
	fm := NewForwardManager(sm).(*forwardManager)
	af := &activeForward{
		session: core.ForwardSession{
			Rule: core.ForwardRule{Name: "test"},
		},
	}

	go fm.handleSOCKS5(af, serverConn, mockDialer)

	// 1バイトずつ送信: Greeting
	for _, b := range []byte{0x05, 0x01, 0x00} {
		_, _ = clientConn.Write([]byte{b})
		time.Sleep(time.Millisecond)
	}

	resp := make([]byte, 2)
	_, _ = io.ReadFull(clientConn, resp)

	// 1バイトずつ送信: Request (domain "a.b" port 80)
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
