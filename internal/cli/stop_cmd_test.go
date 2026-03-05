package cli

import (
	"strings"
	"testing"
)

func TestRunStop_NameRequiredWithoutAll(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunStop("/tmp", []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunStop_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunStop(configDir, []string{"my-forward"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunStop_DaemonNotRunning_All(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, _ := captureExit(t, func() {
		RunStop(configDir, []string{"--all"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunStop_InvalidFlag(t *testing.T) {
	stubExit(t)

	code, stderr := captureExit(t, func() {
		RunStop(t.TempDir(), []string{"--bad-flag"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "bad-flag") {
		t.Errorf("stderr should mention bad-flag, got %q", stderr)
	}
}

func TestRunStop_MockDaemon_ByName(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunStop("", []string{"my-forward"})
	})

	if output == "" {
		t.Error("RunStop should produce output with mock daemon")
	}
}

func TestRunStop_MockDaemon_All(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunStop("", []string{"--all"})
	})

	if output == "" {
		t.Error("RunStop --all should produce output with mock daemon")
	}
}
