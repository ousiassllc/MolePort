package infra

import (
	"context"
	"testing"
)

func TestNewSSHConnection_IsAliveReturnsFalse(t *testing.T) {
	conn := NewSSHConnection()
	if conn.IsAlive() {
		t.Error("IsAlive should return false when not connected")
	}
}

func TestSSHConnection_CloseNilIsSafe(t *testing.T) {
	conn := NewSSHConnection()
	// Close on a connection that was never opened should not panic or error
	if err := conn.Close(); err != nil {
		t.Errorf("Close on nil connection returned error: %v", err)
	}
}

func TestSSHConnection_CloseMultipleTimes(t *testing.T) {
	conn := NewSSHConnection()
	// Multiple Close calls should be safe
	for i := 0; i < 3; i++ {
		if err := conn.Close(); err != nil {
			t.Errorf("Close call %d returned error: %v", i+1, err)
		}
	}
}

func TestSSHConnection_LocalForwardNotConnected(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	_, err := conn.LocalForward(ctx, 8080, "localhost:80")
	if err == nil {
		t.Error("LocalForward should return error when not connected")
	}
}

func TestSSHConnection_RemoteForwardNotConnected(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	_, err := conn.RemoteForward(ctx, 8080, "localhost:80")
	if err == nil {
		t.Error("RemoteForward should return error when not connected")
	}
}

func TestSSHConnection_DynamicForwardNotConnected(t *testing.T) {
	conn := NewSSHConnection()
	ctx := context.Background()
	_, err := conn.DynamicForward(ctx, 1080)
	if err == nil {
		t.Error("DynamicForward should return error when not connected")
	}
}
