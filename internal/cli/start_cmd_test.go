package cli

import (
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/ipc/client"
)

func TestRunStart_NameRequired(t *testing.T) {
	stubExit(t)

	code, _ := captureExit(t, func() {
		RunStart("/tmp", []string{})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunStart_DaemonNotRunning(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()

	code, stderr := captureExit(t, func() {
		RunStart(configDir, []string{"my-forward"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunStart_MockDaemon(t *testing.T) {
	stubConnectDaemon(t)

	output := captureStdout(t, func() {
		RunStart("", []string{"my-forward"})
	})

	if output == "" {
		t.Error("RunStart should produce output with mock daemon")
	}
}

func TestRunStart_SetsCredentialHandler(t *testing.T) {
	stubConnectDaemon(t)
	inner := ConnectDaemon
	var captured *client.IPCClient
	ConnectDaemon = func(configDir string) *client.IPCClient {
		captured = inner(configDir)
		return captured
	}

	captureStdout(t, func() {
		RunStart("", []string{"my-forward"})
	})

	if captured == nil {
		t.Fatal("ConnectDaemon was not called")
	}
	if captured.CredentialHandler() == nil {
		t.Error("RunStart should set a credential handler so that auth prompts can be answered")
	}
}

func TestDaemonCallWithCredentials_Timeout(t *testing.T) {
	stubConnectDaemon(t)

	_, ctx, cleanup := DaemonCallWithCredentials("")
	defer cleanup()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context should have a deadline")
	}
	if remaining := time.Until(deadline); remaining <= defaultCallTimeout || remaining > connectCallTimeout {
		t.Errorf("remaining = %v, want in (%v, %v]", remaining, defaultCallTimeout, connectCallTimeout)
	}
}
