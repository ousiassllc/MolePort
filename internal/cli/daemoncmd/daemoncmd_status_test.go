package daemoncmd

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// serveDaemonStatus は configDir にモックのデーモン（PID ファイルとソケット）を置き、
// daemon.status に status を返す。
func serveDaemonStatus(t *testing.T, status protocol.DaemonStatusResult) string {
	t.Helper()
	configDir := t.TempDir()
	writeFakePID(t, configDir)
	ln, err := net.Listen("unix", filepath.Join(configDir, "moleport.sock"))
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	result, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				scanner := bufio.NewScanner(conn)
				enc := json.NewEncoder(conn)
				for scanner.Scan() {
					var req protocol.Request
					if json.Unmarshal(scanner.Bytes(), &req) != nil {
						return
					}
					if enc.Encode(protocol.Response{JSONRPC: protocol.JSONRPCVersion, ID: req.ID, Result: result}) != nil {
						return
					}
				}
			}()
		}
	}()
	return configDir
}

func TestRunDaemonStatus_PrintsWarnings(t *testing.T) {
	warnings := []string{"config dir permissions too open", "update_check.interval 0s is invalid"}
	configDir := serveDaemonStatus(t, protocol.DaemonStatusResult{Version: "1.2.3", PID: 4242, Warnings: warnings})

	output := captureStdout(t, func() { runDaemonStatus(configDir) })

	for _, w := range warnings {
		want := i18n.T("cli.daemon.status_warning", map[string]any{"Message": w})
		if !strings.Contains(output, want) {
			t.Errorf("output should contain warning line %q, got %q", want, output)
		}
	}
}

func TestRunDaemonStatus_NoWarnings(t *testing.T) {
	configDir := serveDaemonStatus(t, protocol.DaemonStatusResult{Version: "1.2.3", PID: 4242})

	output := captureStdout(t, func() { runDaemonStatus(configDir) })

	if !strings.Contains(output, "4242") {
		t.Errorf("output should contain PID, got %q", output)
	}
	marker := strings.TrimSpace(i18n.T("cli.daemon.status_warning", map[string]any{"Message": ""}))
	if strings.Contains(output, marker) {
		t.Errorf("output should not contain a warning line, got %q", output)
	}
}
