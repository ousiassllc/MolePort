package sshconfig

import "testing"

func TestSSHConfigParser_ProxyCommand(t *testing.T) {
	path := writeSSHConfig(t, `
Host bastion-target
    HostName 10.0.0.5
    ProxyCommand ssh -W %h:%p bastion.example.com
`)

	parser := NewSSHConfigParser()
	hosts, err := parser.Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("len(hosts) = %d, want 1", len(hosts))
	}

	want := "ssh -W %h:%p bastion.example.com"
	if hosts[0].ProxyCommand != want {
		t.Errorf("ProxyCommand = %q, want %q", hosts[0].ProxyCommand, want)
	}
}

func TestSSHConfigParser_NoProxyCommand(t *testing.T) {
	path := writeSSHConfig(t, `
Host noproxycmd
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

	if hosts[0].ProxyCommand != "" {
		t.Errorf("ProxyCommand = %q, want empty string", hosts[0].ProxyCommand)
	}
}

func TestSSHConfigParser_ProxyCommandAndProxyJump(t *testing.T) {
	path := writeSSHConfig(t, `
Host dual-proxy
    HostName 10.0.0.5
    ProxyCommand ssh -W %h:%p bastion.example.com
    ProxyJump jumphost
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
	wantCmd := "ssh -W %h:%p bastion.example.com"
	if h.ProxyCommand != wantCmd {
		t.Errorf("ProxyCommand = %q, want %q", h.ProxyCommand, wantCmd)
	}
	if len(h.ProxyJump) != 1 || h.ProxyJump[0] != "jumphost" {
		t.Errorf("ProxyJump = %v, want [jumphost]", h.ProxyJump)
	}
}
