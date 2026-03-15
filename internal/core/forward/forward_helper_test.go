package forward

import (
	"context"
	"net"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/forwardtest"
)

func TestOpenListener_Remote_PassesRemoteBindAddr(t *testing.T) {
	tests := []struct {
		name           string
		remoteBindAddr string
		wantBindAddr   string
	}{
		{"custom bind addr", "0.0.0.0", "0.0.0.0"},
		{"empty passthrough to sshconn", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBindAddr string
			conn := &forwardtest.MockSSHConnection{
				Alive: true,
				RemoteForwardF: func(_ context.Context, _ int, _ string, remoteBindAddr string) (net.Listener, error) {
					gotBindAddr = remoteBindAddr
					return forwardtest.NewMockListener(), nil
				},
			}

			rule := core.ForwardRule{
				Name:           "test-remote",
				Host:           "server",
				Type:           core.Remote,
				LocalPort:      3000,
				RemotePort:     8080,
				RemoteBindAddr: tt.remoteBindAddr,
			}

			ln, err := openListener(context.Background(), conn, rule)
			if err != nil {
				t.Fatalf("openListener() error = %v", err)
			}
			defer func() { _ = ln.Close() }()

			if gotBindAddr != tt.wantBindAddr {
				t.Errorf("remoteBindAddr passed to RemoteForward = %q, want %q", gotBindAddr, tt.wantBindAddr)
			}
		})
	}
}
