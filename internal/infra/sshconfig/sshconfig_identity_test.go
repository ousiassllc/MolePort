package sshconfig

import (
	"os/user"
	"path/filepath"
	"testing"
)

func TestSSHConfigParser_TildeExpansion(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	path := writeSSHConfig(t, `
Host tildehost
    HostName example.com
    IdentityFile ~/.ssh/id_ed25519
`)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("len(hosts) = %d, want 1", len(hosts))
	}

	expected := filepath.Join(u.HomeDir, ".ssh/id_ed25519")
	if len(hosts[0].IdentityFiles) != 1 || hosts[0].IdentityFiles[0] != expected {
		t.Errorf("IdentityFiles = %v, want [%s]", hosts[0].IdentityFiles, expected)
	}
}

func TestSSHConfigParser_MultipleIdentityFiles(t *testing.T) {
	path := writeSSHConfig(t, `
Host multi-key
    HostName example.com
    User admin
    IdentityFile /home/user/.ssh/id_rsa
    IdentityFile /home/user/.ssh/id_ed25519
`)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("len(hosts) = %d, want 1", len(hosts))
	}

	h := hosts[0]
	if len(h.IdentityFiles) != 2 {
		t.Fatalf("len(IdentityFiles) = %d, want 2", len(h.IdentityFiles))
	}
	if h.IdentityFiles[0] != "/home/user/.ssh/id_rsa" {
		t.Errorf("IdentityFiles[0] = %q, want %q", h.IdentityFiles[0], "/home/user/.ssh/id_rsa")
	}
	if h.IdentityFiles[1] != "/home/user/.ssh/id_ed25519" {
		t.Errorf("IdentityFiles[1] = %q, want %q", h.IdentityFiles[1], "/home/user/.ssh/id_ed25519")
	}
}
