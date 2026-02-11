package infra

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func writeSSHConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write ssh config: %v", err)
	}
	return path
}

func TestSSHConfigParser_BasicHost(t *testing.T) {
	path := writeSSHConfig(t, `
Host myserver
    HostName 192.168.1.10
    Port 2222
    User deploy
    IdentityFile /home/user/.ssh/id_rsa
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
	if h.Name != "myserver" {
		t.Errorf("Name = %q, want %q", h.Name, "myserver")
	}
	if h.HostName != "192.168.1.10" {
		t.Errorf("HostName = %q, want %q", h.HostName, "192.168.1.10")
	}
	if h.Port != 2222 {
		t.Errorf("Port = %d, want 2222", h.Port)
	}
	if h.User != "deploy" {
		t.Errorf("User = %q, want %q", h.User, "deploy")
	}
	if h.IdentityFile != "/home/user/.ssh/id_rsa" {
		t.Errorf("IdentityFile = %q, want %q", h.IdentityFile, "/home/user/.ssh/id_rsa")
	}
	if h.State != core.Disconnected {
		t.Errorf("State = %v, want Disconnected", h.State)
	}
	if h.ActiveForwardCount != 0 {
		t.Errorf("ActiveForwardCount = %d, want 0", h.ActiveForwardCount)
	}
}

func TestSSHConfigParser_WildcardExclusion(t *testing.T) {
	path := writeSSHConfig(t, `
Host *
    ServerAliveInterval 60

Host myserver
    HostName 10.0.0.1
`)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("len(hosts) = %d, want 1", len(hosts))
	}
	if hosts[0].Name != "myserver" {
		t.Errorf("Name = %q, want %q", hosts[0].Name, "myserver")
	}
}

func TestSSHConfigParser_DefaultValues(t *testing.T) {
	path := writeSSHConfig(t, `
Host simple
    HostName example.com
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
	if h.Port != 22 {
		t.Errorf("Port = %d, want 22 (default)", h.Port)
	}

	u, _ := user.Current()
	if u != nil && h.User != u.Username {
		t.Errorf("User = %q, want %q (current user)", h.User, u.Username)
	}
}

func TestSSHConfigParser_MultipleHosts(t *testing.T) {
	path := writeSSHConfig(t, `
Host server1
    HostName 10.0.0.1

Host server2
    HostName 10.0.0.2

Host server3
    HostName 10.0.0.3
`)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 3 {
		t.Fatalf("len(hosts) = %d, want 3", len(hosts))
	}

	names := make(map[string]bool)
	for _, h := range hosts {
		names[h.Name] = true
	}
	for _, name := range []string{"server1", "server2", "server3"} {
		if !names[name] {
			t.Errorf("missing host %q", name)
		}
	}
}

func TestSSHConfigParser_ProxyJump(t *testing.T) {
	path := writeSSHConfig(t, `
Host target
    HostName 10.0.0.5
    ProxyJump bastion1,bastion2
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
	if len(h.ProxyJump) != 2 {
		t.Fatalf("len(ProxyJump) = %d, want 2", len(h.ProxyJump))
	}
	if h.ProxyJump[0] != "bastion1" {
		t.Errorf("ProxyJump[0] = %q, want %q", h.ProxyJump[0], "bastion1")
	}
	if h.ProxyJump[1] != "bastion2" {
		t.Errorf("ProxyJump[1] = %q, want %q", h.ProxyJump[1], "bastion2")
	}
}

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
	if hosts[0].IdentityFile != expected {
		t.Errorf("IdentityFile = %q, want %q", hosts[0].IdentityFile, expected)
	}
}

func TestSSHConfigParser_MissingHostNameUsesAlias(t *testing.T) {
	path := writeSSHConfig(t, `
Host aliashost
    Port 2222
`)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("len(hosts) = %d, want 1", len(hosts))
	}

	if hosts[0].HostName != "aliashost" {
		t.Errorf("HostName = %q, want %q (should use alias)", hosts[0].HostName, "aliashost")
	}
}

func TestSSHConfigParser_NonexistentFile(t *testing.T) {
	parser := NewSSHConfigParser()
	_, err := parser.Parse("/nonexistent/path/config")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestSSHConfigParser_EmptyConfig(t *testing.T) {
	path := writeSSHConfig(t, ``)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 0 {
		t.Errorf("len(hosts) = %d, want 0 for empty config", len(hosts))
	}
}

func TestSSHConfigParser_NoProxyJump(t *testing.T) {
	path := writeSSHConfig(t, `
Host noproxy
    HostName 10.0.0.1
`)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("len(hosts) = %d, want 1", len(hosts))
	}

	if hosts[0].ProxyJump != nil {
		t.Errorf("ProxyJump = %v, want nil", hosts[0].ProxyJump)
	}
}
