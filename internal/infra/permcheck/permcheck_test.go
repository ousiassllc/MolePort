package permcheck

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setupPermFixture(t *testing.T, dirMode, fileMode, sshMode os.FileMode) (configDir, sshConfig string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("パーミッション検査は Windows では行わない")
	}
	root := t.TempDir()
	configDir = filepath.Join(root, "moleport")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sshConfig = filepath.Join(root, "ssh_config")
	for _, f := range []struct {
		path string
		mode os.FileMode
	}{
		{filepath.Join(configDir, "config.yaml"), fileMode},
		{filepath.Join(configDir, "state.yaml"), fileMode},
		{sshConfig, sshMode},
	} {
		if err := os.WriteFile(f.path, []byte("x: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		// umask に依存しないよう明示的に設定する。
		if err := os.Chmod(f.path, f.mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(configDir, dirMode); err != nil {
		t.Fatal(err)
	}
	return configDir, sshConfig
}

func TestWarnings_Strict(t *testing.T) {
	configDir, sshConfig := setupPermFixture(t, 0o700, 0o600, 0o600)
	if got := Warnings(configDir, sshConfig); len(got) != 0 {
		t.Errorf("Warnings() = %v, want none", got)
	}
}

func TestWarnings_Loose(t *testing.T) {
	configDir, sshConfig := setupPermFixture(t, 0o755, 0o644, 0o644)

	got := Warnings(configDir, sshConfig)

	for _, want := range []string{
		configDir + " has permissions 0755",
		filepath.Join(configDir, "config.yaml") + " has permissions 0644",
		filepath.Join(configDir, "state.yaml") + " has permissions 0644",
		sshConfig + " has permissions 0644",
	} {
		if !containsSubstring(got, want) {
			t.Errorf("warnings %v do not contain %q", got, want)
		}
	}
	if len(got) != 4 {
		t.Errorf("len(warnings) = %d, want 4: %v", len(got), got)
	}
}

func TestWarnings_MissingPathsAreIgnored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("パーミッション検査は Windows では行わない")
	}
	dir := filepath.Join(t.TempDir(), "none")
	if got := Warnings(dir, filepath.Join(dir, "ssh_config")); len(got) != 0 {
		t.Errorf("Warnings() = %v, want none", got)
	}
}

func containsSubstring(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
