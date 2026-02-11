package daemon

import (
	"os"
	"testing"
)

func TestIsDaemonMode_NotPresent(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport", "daemon", "start"}

	if IsDaemonMode() {
		t.Error("IsDaemonMode() = true, want false")
	}
}

func TestIsDaemonMode_Present(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport", "--daemon-mode", "--config-dir", "/tmp/test"}

	if !IsDaemonMode() {
		t.Error("IsDaemonMode() = false, want true")
	}
}

func TestIsDaemonMode_Empty(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport"}

	if IsDaemonMode() {
		t.Error("IsDaemonMode() = true, want false for empty args")
	}
}
