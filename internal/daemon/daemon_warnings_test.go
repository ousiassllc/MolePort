package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func hasWarning(warnings []string, sub string) bool {
	for _, w := range warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

// config.yaml のパーミッションが緩いと daemon.status の warnings に載り、0600 では載らない（NFR-10）。
func TestDaemon_New_WarnsOnLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("パーミッション検査は Windows では行わない")
	}
	dir := createTestConfigDir(t)
	cfgPath := filepath.Join(dir, "config.yaml")

	strict, err := New(dir, "test")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer strict.cancel()
	if hasWarning(strict.Status().Warnings, cfgPath+" has permissions") {
		t.Errorf("0600 の config.yaml に権限警告が出た: %v", strict.Status().Warnings)
	}

	if err := os.Chmod(cfgPath, 0o644); err != nil { //nolint:gosec // 緩い権限での警告を検証するため
		t.Fatal(err)
	}
	loose, err := New(dir, "test")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer loose.cancel()
	if !hasWarning(loose.Status().Warnings, cfgPath+" has permissions 0644") {
		t.Errorf("Status().Warnings = %v, want config.yaml permission warning", loose.Status().Warnings)
	}
}

// update_check.interval が 0 以下・1h 未満の config.yaml でも New は成功し、補正の警告が載る（NFR-31）。
func TestDaemon_New_WarnsOnInvalidUpdateInterval(t *testing.T) {
	for _, interval := range []string{"0s", "-1h", "30m"} {
		t.Run(interval, func(t *testing.T) {
			dir := t.TempDir()
			yaml := "update_check:\n  enabled: true\n  interval: " + interval + "\n"
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			d, err := New(dir, "v1.0.0")
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			defer d.cancel()
			if !hasWarning(d.Status().Warnings, "update_check.interval") {
				t.Errorf("Status().Warnings = %v, want interval warning", d.Status().Warnings)
			}
		})
	}
}
