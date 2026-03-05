package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunVersion_BasicOutput(t *testing.T) {
	configDir := t.TempDir()

	output := captureStdout(t, func() {
		RunVersion(configDir, []string{})
	})

	if !strings.Contains(output, "MolePort") {
		t.Errorf("output should contain 'MolePort', got %q", output)
	}
	if !strings.Contains(output, Version) {
		t.Errorf("output should contain version %q, got %q", Version, output)
	}
	if !strings.Contains(output, runtime.Version()) {
		t.Errorf("output should contain Go version %q, got %q", runtime.Version(), output)
	}
	if !strings.Contains(output, runtime.GOOS) {
		t.Errorf("output should contain GOOS %q, got %q", runtime.GOOS, output)
	}
}

func TestRunVersion_NoDaemon_NoPanic(t *testing.T) {
	configDir := t.TempDir()

	output := captureStdout(t, func() {
		RunVersion(configDir, []string{})
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line of output, got %d: %q", len(lines), output)
	}
}

func TestRunVersion_ContainsOSAndArch(t *testing.T) {
	configDir := t.TempDir()

	output := captureStdout(t, func() {
		RunVersion(configDir, []string{})
	})

	if !strings.Contains(output, runtime.GOARCH) {
		t.Errorf("output should contain GOARCH %q, got %q", runtime.GOARCH, output)
	}
}

func TestRunVersion_IgnoresExtraArgs(t *testing.T) {
	output := captureStdout(t, func() {
		RunVersion(t.TempDir(), []string{"--extra"})
	})

	if !strings.Contains(output, "MolePort") {
		t.Errorf("output should contain 'MolePort', got %q", output)
	}
}

func TestRunVersion_DaemonRunningButNoSocket(t *testing.T) {
	configDir := t.TempDir()
	pidPath := filepath.Join(configDir, "moleport.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	output := captureStdout(t, func() {
		RunVersion(configDir, []string{})
	})

	if !strings.Contains(output, "MolePort") {
		t.Errorf("output should contain 'MolePort', got %q", output)
	}
}
