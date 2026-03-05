package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// exitCalled はテスト用 ExitFunc が呼ばれたことを示す panic 型。
type exitCalled struct{ code int }

// stubExit は ExitFunc を差し替えて os.Exit を回避するヘルパー。
func stubExit(t *testing.T) {
	t.Helper()
	orig := ExitFunc
	t.Cleanup(func() { ExitFunc = orig })
	ExitFunc = func(c int) { panic(exitCalled{code: c}) }
}

// captureExit は fn を呼び、ExitFunc 経由の終了コードと stderr 出力を返す。
func captureExit(t *testing.T, fn func()) (code int, stderr string) {
	t.Helper()
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = w.Close()
		_ = r.Close()
		os.Stderr = origStderr
	})
	os.Stderr = w
	code = -1
	func() {
		defer func() {
			if v := recover(); v != nil {
				if ec, ok := v.(exitCalled); ok {
					code = ec.code
				} else {
					panic(v)
				}
			}
		}()
		fn()
	}()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return code, buf.String()
}

// captureStdout は fn 実行中の stdout 出力をキャプチャして返す。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

// stubConnectDaemon は ConnectDaemon を差し替えてモック IPC サーバーに接続するヘルパー。
// 全ての RPC 呼び出しに対して空の成功レスポンス ({}) を返す。
func stubConnectDaemon(t *testing.T) {
	t.Helper()
	orig := ConnectDaemon
	t.Cleanup(func() { ConnectDaemon = orig })

	sockPath := filepath.Join(t.TempDir(), "mock.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleMockConn(conn)
		}
	}()

	ConnectDaemon = func(_ string) *client.IPCClient {
		c := client.NewIPCClient(sockPath)
		if err := c.Connect(); err != nil {
			t.Fatalf("mock connect: %v", err)
		}
		return c
	}
}

func handleMockConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	enc := json.NewEncoder(conn)
	for scanner.Scan() {
		var req protocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return
		}
		resp := protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
}
