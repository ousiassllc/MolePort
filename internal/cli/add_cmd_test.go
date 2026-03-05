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

func TestRunAdd_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunAdd(configDir, []string{
			"--host", "myserver",
			"--local-port", "8080",
			"--remote-port", "80",
		})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunAdd_DynamicType_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, _ := captureExit(t, func() {
		RunAdd(configDir, []string{
			"--host", "myserver",
			"--local-port", "1080",
			"--type", "dynamic",
		})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunAdd_InvalidFlag(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunAdd(t.TempDir(), []string{"--bad-flag"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunAdd_MockDaemon_Local(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunAdd("", []string{
			"--host", "myserver",
			"--local-port", "8080",
			"--remote-port", "80",
		})
	})

	if output == "" {
		t.Error("RunAdd should produce output with mock daemon")
	}
}

func TestRunAdd_MockDaemon_Dynamic(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunAdd("", []string{
			"--host", "myserver",
			"--local-port", "1080",
			"--type", "dynamic",
		})
	})

	if output == "" {
		t.Error("RunAdd dynamic should produce output with mock daemon")
	}
}
