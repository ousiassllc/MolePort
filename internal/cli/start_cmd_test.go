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
