package cli

import (
	"strings"
	"testing"
)

func TestRunConfig_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunConfig(configDir, []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunConfig_DaemonNotRunning_JSON(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, _ := captureExit(t, func() {
		RunConfig(configDir, []string{"-json"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunConfig_InvalidFlag(t *testing.T) {
	stubExit(t)

	code, stderr := captureExit(t, func() {
		RunConfig(t.TempDir(), []string{"--invalid-flag"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid") {
		t.Errorf("stderr should mention invalid flag, got %q", stderr)
	}
}

func TestRunConfig_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunConfig("", []string{})
	})

	if !strings.Contains(output, "MolePort Config") {
		t.Errorf("output should contain 'MolePort Config', got %q", output)
	}
}

func TestRunConfig_MockDaemon_JSON(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunConfig("", []string{"-json"})
	})

	// JSON 出力にはブレースが含まれる
	if !strings.Contains(output, "{") {
		t.Errorf("JSON output should contain '{', got %q", output)
	}
}
