package cli

import (
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestPrintForwardLine_Local(t *testing.T) {
	f := protocol.ForwardInfo{
		Type:       protocol.ForwardTypeLocal,
		LocalPort:  8080,
		RemoteHost: "remote",
		RemotePort: 80,
	}

	output := captureStdout(t, func() {
		printForwardLine(f)
	})

	if !strings.Contains(output, "L") {
		t.Errorf("local forward should show 'L', got %q", output)
	}
	if !strings.Contains(output, "8080") {
		t.Errorf("should show local port 8080, got %q", output)
	}
	if !strings.Contains(output, "remote:80") {
		t.Errorf("should show remote:80, got %q", output)
	}
}

func TestPrintForwardLine_Remote(t *testing.T) {
	f := protocol.ForwardInfo{
		Type:       protocol.ForwardTypeRemote,
		LocalPort:  9090,
		RemoteHost: "host",
		RemotePort: 3000,
	}

	output := captureStdout(t, func() {
		printForwardLine(f)
	})

	if !strings.Contains(output, "R") {
		t.Errorf("remote forward should show 'R', got %q", output)
	}
}

func TestPrintForwardLine_Dynamic(t *testing.T) {
	f := protocol.ForwardInfo{
		Type:      protocol.ForwardTypeDynamic,
		LocalPort: 1080,
	}

	output := captureStdout(t, func() {
		printForwardLine(f)
	})

	if !strings.Contains(output, "D") {
		t.Errorf("dynamic forward should show 'D', got %q", output)
	}
	if !strings.Contains(output, "1080") {
		t.Errorf("should show local port 1080, got %q", output)
	}
	// dynamic は -> を含まない
	if strings.Contains(output, "->") {
		t.Errorf("dynamic forward should not show '->', got %q", output)
	}
}

func TestRunList_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunList(configDir, []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunList_DaemonNotRunning_JSON(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, _ := captureExit(t, func() {
		RunList(configDir, []string{"-json"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunList_DaemonNotRunning_WithHost(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, _ := captureExit(t, func() {
		RunList(configDir, []string{"-host", "myserver"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunList_InvalidFlag(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunList(t.TempDir(), []string{"--bad-flag"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunList_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	// モックデーモンは空のホスト/フォワードリストを返す
	output := captureStdout(t, func() {
		RunList("", []string{})
	})

	if output == "" {
		t.Error("RunList should produce output with mock daemon")
	}
}

func TestRunList_MockDaemon_JSON(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunList("", []string{"-json"})
	})

	if !strings.Contains(output, "{") {
		t.Errorf("JSON output should contain '{', got %q", output)
	}
}

func TestRunList_MockDaemon_WithHost(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunList("", []string{"-host", "myserver"})
	})

	if output == "" {
		t.Error("RunList should produce output with host filter")
	}
}
