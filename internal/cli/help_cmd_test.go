package cli

import (
	"strings"
	"testing"
)

func TestRunHelp_PrintsUsage(t *testing.T) {
	output := captureStdout(t, func() {
		RunHelp("/tmp", []string{})
	})

	if output == "" {
		t.Error("RunHelp should produce output")
	}
}

func TestRunHelp_ContainsMolePort(t *testing.T) {
	output := captureStdout(t, func() {
		RunHelp("/tmp", []string{})
	})

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "moleport") {
		t.Errorf("help output should mention moleport, got %q", output)
	}
}

func TestRunHelp_IgnoresExtraArgs(t *testing.T) {
	// 追加引数があってもパニックしないこと
	output := captureStdout(t, func() {
		RunHelp("/tmp", []string{"extra", "args"})
	})

	if output == "" {
		t.Error("RunHelp should produce output even with extra args")
	}
}
