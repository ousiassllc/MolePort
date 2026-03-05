package cli

import "testing"

func TestRunStart_NameRequired(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunStart("/tmp", []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunStart_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunStart(configDir, []string{"my-forward"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunStart_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunStart("", []string{"my-forward"})
	})

	if output == "" {
		t.Error("RunStart should produce output with mock daemon")
	}
}
