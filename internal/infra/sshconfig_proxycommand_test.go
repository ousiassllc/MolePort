package infra

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

func TestExpandProxyCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		host    string
		port    int
		user    string
		want    string
	}{
		{
			name:    "basic %h and %p",
			command: "ssh -W %h:%p bastion",
			host:    "example.com",
			port:    22,
			user:    "admin",
			want:    "ssh -W example.com:22 bastion",
		},
		{
			name:    "all tokens %h %p %r",
			command: "connect --host %h --port %p --user %r",
			host:    "db.internal",
			port:    5432,
			user:    "deploy",
			want:    "connect --host db.internal --port 5432 --user deploy",
		},
		{
			name:    "escaped %%",
			command: "echo 100%% > /dev/null && ssh -W %h:%p bastion",
			host:    "target",
			port:    22,
			user:    "root",
			want:    "echo 100% > /dev/null && ssh -W target:22 bastion",
		},
		{
			name:    "no tokens",
			command: "nc proxy.example.com 8080",
			host:    "target",
			port:    22,
			user:    "root",
			want:    "nc proxy.example.com 8080",
		},
		{
			name:    "empty string",
			command: "",
			host:    "target",
			port:    22,
			user:    "root",
			want:    "",
		},
		{
			name:    "unknown token %x preserved",
			command: "cmd %x %h",
			host:    "target",
			port:    22,
			user:    "root",
			want:    "cmd %x target",
		},
		{
			name:    "trailing percent",
			command: "cmd %h %",
			host:    "target",
			port:    22,
			user:    "root",
			want:    "cmd target %",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandProxyCommand(tt.command, tt.host, tt.port, tt.user)
			if got != tt.want {
				t.Errorf("ExpandProxyCommand(%q, %q, %d, %q) = %q, want %q",
					tt.command, tt.host, tt.port, tt.user, got, tt.want)
			}
		})
	}
}
