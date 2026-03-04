package infra

import "testing"

func TestProxyCommandAddr_NetworkAndString(t *testing.T) {
	desc := "ssh -W %h:%p bastion.example.com"
	addr := proxyCommandAddr{desc: desc}

	if got := addr.Network(); got != "proxycommand" {
		t.Errorf("Network() = %q, want %q", got, "proxycommand")
	}

	if got := addr.String(); got != desc {
		t.Errorf("String() = %q, want %q", got, desc)
	}
}
