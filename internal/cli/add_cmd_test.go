package cli

import "testing"

func TestRunAdd_HostRequired(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunAdd("/tmp", []string{"--local-port", "8080"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunAdd_LocalPortRequired(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunAdd("/tmp", []string{"--host", "myserver"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunAdd_PortRangeInvalid(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunAdd("/tmp", []string{"--host", "myserver", "--local-port", "99999"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunAdd_InvalidType(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunAdd("/tmp", []string{
			"--host", "myserver", "--local-port", "8080", "--type", "invalid",
		})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunAdd_RemotePortRequiredForLocal(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunAdd("/tmp", []string{
			"--host", "myserver", "--local-port", "8080", "--type", "local",
		})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunAdd_RemotePortRangeInvalid(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunAdd("/tmp", []string{
			"--host", "myserver",
			"--local-port", "8080",
			"--type", "remote",
			"--remote-port", "70000",
		})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}
