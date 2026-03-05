package infra

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestProxyCommandConn_ReadWrite(t *testing.T) {
	conn, err := dialViaProxyCommand("cat")
	if err != nil {
		t.Fatalf("dialViaProxyCommand: %v", err)
	}
	defer func() { _ = conn.Close() }()

	msg := []byte("hello proxy")
	n, err := conn.Write(msg)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("Write: wrote %d bytes, want %d", n, len(msg))
	}

	buf := make([]byte, len(msg))
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if string(buf) != string(msg) {
		t.Errorf("Read = %q, want %q", buf, msg)
	}
}

func TestProxyCommandConn_Close(t *testing.T) {
	conn, err := dialViaProxyCommand("cat")
	if err != nil {
		t.Fatalf("dialViaProxyCommand: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = conn.Write([]byte("after close"))
	if err == nil {
		t.Error("Write after Close should return error")
	}
}

func TestProxyCommandConn_NetConnInterface(t *testing.T) {
	conn, err := dialViaProxyCommand("cat")
	if err != nil {
		t.Fatalf("dialViaProxyCommand: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// net.Conn インターフェース準拠の確認
	var _ net.Conn = conn

	if conn.LocalAddr() == nil {
		t.Error("LocalAddr should not be nil")
	}
	if conn.RemoteAddr() == nil {
		t.Error("RemoteAddr should not be nil")
	}

	if err := conn.SetDeadline(time.Now()); err != nil {
		t.Errorf("SetDeadline: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now()); err != nil {
		t.Errorf("SetReadDeadline: %v", err)
	}
	if err := conn.SetWriteDeadline(time.Now()); err != nil {
		t.Errorf("SetWriteDeadline: %v", err)
	}
}

func TestDialViaProxyCommand_InvalidCommand(t *testing.T) {
	// 存在しないコマンドを sh 経由で実行すると sh 自体は起動成功するため、
	// Read でプロセス終了後の EOF エラーが返ることを検証する。
	conn, err := dialViaProxyCommand("/nonexistent/command")
	if err != nil {
		// sh の起動自体が失敗した場合（環境依存）もエラーとして正しい
		return
	}
	defer func() { _ = conn.Close() }()

	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	if readErr == nil {
		t.Error("Read should return error for invalid command")
	}
}

func TestProxyCommandConn_CloseMultipleTimes(t *testing.T) {
	conn, err := dialViaProxyCommand("cat")
	if err != nil {
		t.Fatalf("dialViaProxyCommand: %v", err)
	}

	// Close の多重呼び出しがパニックせずエラーも返さないことを検証
	for range 3 {
		if err := conn.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	}
}

func TestCommandName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ssh -o ProxyCommand=none host", "ssh"},
		{"nc %h %p", "nc"},
		{"/usr/bin/ssh", "/usr/bin/ssh"},
		{"", ""},
		{" leading-space", "leading-space"},
	}
	for _, tt := range tests {
		got := commandName(tt.input)
		if got != tt.want {
			t.Errorf("commandName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestProxyCommandConn_CloseTerminatesProcess(t *testing.T) {
	conn, err := dialViaProxyCommand("sleep 3600")
	if err != nil {
		t.Fatalf("dialViaProxyCommand: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-conn.done:
		// プロセスが正常に終了した
	case <-time.After(5 * time.Second):
		t.Fatal("process did not terminate within 5 seconds after Close")
	}
}
