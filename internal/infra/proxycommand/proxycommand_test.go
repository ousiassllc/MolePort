package proxycommand

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestConn_ReadWrite(t *testing.T) {
	c, err := Dial("cat")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = c.Close() }()

	msg := []byte("hello proxy")
	n, err := c.Write(msg)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("Write: wrote %d bytes, want %d", n, len(msg))
	}

	buf := make([]byte, len(msg))
	_, err = io.ReadFull(c, buf)
	if err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if string(buf) != string(msg) {
		t.Errorf("Read = %q, want %q", buf, msg)
	}
}

func TestConn_Close(t *testing.T) {
	c, err := Dial("cat")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = c.Write([]byte("after close"))
	if err == nil {
		t.Error("Write after Close should return error")
	}
}

func TestConn_NetConnInterface(t *testing.T) {
	c, err := Dial("cat")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = c.Close() }()

	// net.Conn インターフェース準拠の確認
	var _ = net.Conn(c)

	if c.LocalAddr() == nil {
		t.Error("LocalAddr should not be nil")
	}
	if c.RemoteAddr() == nil {
		t.Error("RemoteAddr should not be nil")
	}

	if err := c.SetDeadline(time.Now()); err != nil {
		t.Errorf("SetDeadline: %v", err)
	}
	if err := c.SetReadDeadline(time.Now()); err != nil {
		t.Errorf("SetReadDeadline: %v", err)
	}
	if err := c.SetWriteDeadline(time.Now()); err != nil {
		t.Errorf("SetWriteDeadline: %v", err)
	}
}

func TestDial_InvalidCommand(t *testing.T) {
	c, err := Dial("/nonexistent/command")
	if err != nil {
		return
	}
	defer func() { _ = c.Close() }()

	buf := make([]byte, 1)
	_, readErr := c.Read(buf)
	if readErr == nil {
		t.Error("Read should return error for invalid command")
	}
}

func TestConn_CloseMultipleTimes(t *testing.T) {
	c, err := Dial("cat")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	for range 3 {
		if err := c.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	}
}

func TestCommandName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ssh -o ProxyCommand=none host", "ssh"},
		{"nc %h %p", "nc"},
		{"/usr/bin/ssh", "/usr/bin/ssh"},
		{"", ""},
		{" leading-space", "leading-space"},
	}
	for _, tt := range tests {
		got := commandName(tt.input)
		if got != tt.want {
			t.Errorf("commandName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConn_CloseTerminatesProcess(t *testing.T) {
	c, err := Dial("sleep 3600")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	cn := c.(*conn) // done チャネルへのアクセス用
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-cn.done:
	case <-time.After(5 * time.Second):
		t.Fatal("process did not terminate within 5 seconds after Close")
	}
}

func TestAddr_NetworkAndString(t *testing.T) {
	desc := "ssh -W %h:%p bastion.example.com"
	a := addr{desc: desc}

	if got := a.Network(); got != "proxycommand" {
		t.Errorf("Network() = %q, want %q", got, "proxycommand")
	}

	if got := a.String(); got != desc {
		t.Errorf("String() = %q, want %q", got, desc)
	}
}

func TestExpandCommand(t *testing.T) {
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
			got := ExpandCommand(tt.command, tt.host, tt.port, tt.user)
			if got != tt.want {
				t.Errorf("ExpandCommand(%q, %q, %d, %q) = %q, want %q",
					tt.command, tt.host, tt.port, tt.user, got, tt.want)
			}
		})
	}
}
