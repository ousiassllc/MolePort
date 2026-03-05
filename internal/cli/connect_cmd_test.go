package cli

import "testing"

func TestRunConnect_HostRequired(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunConnect("/tmp", []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunConnect_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunConnect(configDir, []string{"myhost"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunConnect_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunConnect("", []string{"myhost"})
	})

	if output == "" {
		t.Error("RunConnect should produce output with mock daemon")
	}
}
