package infra

import (
	"context"
	"testing"
	"time"
)

func TestSSHConnection_LocalForwardSuccess(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := conn.LocalForward(ctx, 0, "localhost:80")
	if err != nil {
		t.Fatalf("LocalForward failed: %v", err)
	}
	if ln == nil {
		t.Fatal("LocalForward returned nil listener")
	}
	if ln.Addr().String() == "" {
		t.Error("listener address should not be empty")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	if _, err = ln.Accept(); err == nil {
		t.Error("Accept should fail after context cancellation")
	}
}

func TestSSHConnection_DynamicForwardSuccess(t *testing.T) {
	s := newTestSSHServer(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := conn.DynamicForward(ctx, 0)
	if err != nil {
		t.Fatalf("DynamicForward failed: %v", err)
	}
	if ln == nil {
		t.Fatal("DynamicForward returned nil listener")
	}
	if ln.Addr().String() == "" {
		t.Error("listener address should not be empty")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	if _, err = ln.Accept(); err == nil {
		t.Error("Accept should fail after context cancellation")
	}
}

func TestSSHConnection_RemoteForwardSuccess(t *testing.T) {
	s := newTestSSHServerWithTCPIPForward(t)
	conn := dialTestServer(t, s, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := conn.RemoteForward(ctx, 0, "localhost:80")
	if err != nil {
		t.Fatalf("RemoteForward failed: %v", err)
	}
	if ln == nil {
		t.Fatal("RemoteForward returned nil listener")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)
	if _, err = ln.Accept(); err == nil {
		t.Error("Accept should fail after context cancellation")
	}
}
