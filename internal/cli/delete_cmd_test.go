package cli

import "testing"

func TestRunDelete_NameRequired(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunDelete("/tmp", []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunDelete_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunDelete(configDir, []string{"my-forward"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunDelete_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunDelete("", []string{"my-forward"})
	})

	if output == "" {
		t.Error("RunDelete should produce output with mock daemon")
	}
}
