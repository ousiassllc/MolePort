package cli

import (
	"testing"
)

func TestRunTUI_DaemonStartFails(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	// daemon が起動できない環境（configDir は一時ディレクトリ）では
	// StartDaemonProcess がエラーを返し、exitError が呼ばれる
	code, stderr := captureExit(t, func() {
		RunTUI(configDir, []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}
