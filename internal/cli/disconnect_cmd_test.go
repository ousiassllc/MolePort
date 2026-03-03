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
