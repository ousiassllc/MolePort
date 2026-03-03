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
