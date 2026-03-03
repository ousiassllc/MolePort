package cli

import "testing"

func TestRunStop_NameRequiredWithoutAll(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunStop("/tmp", []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}
