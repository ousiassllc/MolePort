package infra

import (
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"~/.ssh/config", filepath.Join(u.HomeDir, ".ssh/config"), false},
		{"~/", u.HomeDir, false},
		{"~", u.HomeDir, false},
		{"~otheruser/.ssh/config", "~otheruser/.ssh/config", false},
		{"~otheruser", "~otheruser", false},
		{"/absolute/path", "/absolute/path", false},
		{"relative/path", "relative/path", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got, err := expandTilde(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("expandTilde(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDefaultKeyPaths(t *testing.T) {
	paths := defaultKeyPaths()
	if len(paths) == 0 {
		t.Fatal("defaultKeyPaths returned empty slice")
	}

	expectedNames := []string{"id_rsa", "id_ed25519", "id_ecdsa", "id_dsa"}
	for i, name := range expectedNames {
		if i >= len(paths) {
			t.Errorf("missing key path for %s", name)
			continue
		}
		if !strings.HasSuffix(paths[i], name) {
			t.Errorf("paths[%d] = %q, want suffix %q", i, paths[i], name)
		}
		if !strings.Contains(paths[i], ".ssh") {
			t.Errorf("paths[%d] = %q, should contain .ssh", i, paths[i])
		}
	}
}
