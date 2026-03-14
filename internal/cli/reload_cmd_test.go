package cli

import (
	"testing"
)

func TestRunReload_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunReload(configDir, []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunReload_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunReload("", []string{})
	})

	if output == "" {
		t.Error("RunReload should produce output with mock daemon")
	}
}
