package version

import "testing"

func TestVersion_DefaultIsDev(t *testing.T) {
	if Version != "dev" {
		t.Errorf("Version = %q, want %q (ldflags 未指定のデフォルト)", Version, "dev")
	}
}
