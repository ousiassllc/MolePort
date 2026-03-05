package statuscmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/cli"
	"github.com/ousiassllc/moleport/internal/format"
	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

type exitCalled struct{ code int }

func stubExit(t *testing.T) {
	t.Helper()
	orig := cli.ExitFunc
	t.Cleanup(func() { cli.ExitFunc = orig })
	cli.ExitFunc = func(c int) { panic(exitCalled{code: c}) }
}

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

func stubConnectDaemon(t *testing.T) {
	t.Helper()
	orig := cli.ConnectDaemon
	t.Cleanup(func() { cli.ConnectDaemon = orig })

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

	cli.ConnectDaemon = func(_ string) *client.IPCClient {
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
		if err := enc.Encode(protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}); err != nil {
			return
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0B"},
		{100, "100B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1048576, "1.0MB"},
		{1572864, "1.5MB"},
		{1073741824, "1.0GB"},
		{1610612736, "1.5GB"},
	}
	for _, tt := range tests {
		got := format.Bytes(tt.input)
		if got != tt.want {
			t.Errorf("format.Bytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRunStatus_NoDaemon_PrintsNotRunning(t *testing.T) {
	output := captureStdout(t, func() { RunStatus(t.TempDir(), []string{}) })
	if output == "" {
		t.Error("RunStatus should produce output when daemon is not running")
	}
}

func TestRunStatus_SessionGet_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	code, stderr := captureExit(t, func() { RunStatus(t.TempDir(), []string{"my-session"}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunStatus_InvalidFlag(t *testing.T) {
	stubExit(t)
	code, stderr := captureExit(t, func() { RunStatus(t.TempDir(), []string{"--bad-flag"}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "bad-flag") {
		t.Errorf("stderr should mention bad-flag, got %q", stderr)
	}
}

func TestRunStatus_Summary_DaemonRunningButNoSocket(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()
	pidPath := filepath.Join(configDir, "moleport.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		t.Fatalf("write PID file: %v", err)
	}
	code, stderr := captureExit(t, func() { RunStatus(configDir, []string{}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunStatus_SessionGet_JSON_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	code, _ := captureExit(t, func() { RunStatus(t.TempDir(), []string{"-json", "my-session"}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunStatus_SessionGet_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)
	output := captureStdout(t, func() { RunStatus("", []string{"my-session"}) })
	if output == "" {
		t.Error("RunStatus session get should produce output")
	}
}

func TestRunStatus_SessionGet_JSON_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)
	output := captureStdout(t, func() { RunStatus("", []string{"-json", "my-session"}) })
	if !strings.Contains(output, "{") {
		t.Errorf("JSON output should contain '{', got %q", output)
	}
}
