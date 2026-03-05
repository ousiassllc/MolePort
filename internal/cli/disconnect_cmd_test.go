package cli

import "testing"

func TestRunDisconnect_HostRequired(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunDisconnect("/tmp", []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunDisconnect_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunDisconnect(configDir, []string{"myhost"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunDisconnect_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunDisconnect("", []string{"myhost"})
	})

	if output == "" {
		t.Error("RunDisconnect should produce output with mock daemon")
	}
}
