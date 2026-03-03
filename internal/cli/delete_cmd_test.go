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
