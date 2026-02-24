package ssh

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestSSHManager_LoadHosts(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, nil)

	loaded, err := sm.LoadHosts()
	if err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("len(hosts) = %d, want 2", len(loaded))
	}
	if loaded[0].Name != "server1" {
		t.Errorf("hosts[0].Name = %q, want %q", loaded[0].Name, "server1")
	}
	if loaded[1].Name != "server2" {
		t.Errorf("hosts[1].Name = %q, want %q", loaded[1].Name, "server2")
	}
}

func TestSSHManager_LoadHosts_ParseError(t *testing.T) {
	parser := &mockSSHConfigParser{err: fmt.Errorf("parse error")}
	sm := NewSSHManager(parser, nil, "/fake/ssh/config", core.ReconnectConfig{})

	_, err := sm.LoadHosts()
	if err == nil {
		t.Fatal("LoadHosts() should return error on parse failure")
	}
}

func TestSSHManager_GetHost(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, nil)

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	host, err := sm.GetHost("server1")
	if err != nil {
		t.Fatalf("GetHost() error = %v", err)
	}
	if host.Name != "server1" {
		t.Errorf("host.Name = %q, want %q", host.Name, "server1")
	}
	if host.HostName != "192.168.1.1" {
		t.Errorf("host.HostName = %q, want %q", host.HostName, "192.168.1.1")
	}
}

func TestSSHManager_GetHost_NotFound(t *testing.T) {
	sm := newTestSSHManager(testHosts(), nil)
	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	_, err := sm.GetHost("nonexistent")
	if err == nil {
		t.Fatal("GetHost() should return error for nonexistent host")
	}
}

func TestSSHManager_ReloadHosts_PreservesState(t *testing.T) {
	hosts := testHosts()
	sm := newTestSSHManager(hosts, func() core.SSHConnection {
		return &mockSSHConnection{client: nil, isAlive: true}
	})

	if _, err := sm.LoadHosts(); err != nil {
		t.Fatalf("LoadHosts() error = %v", err)
	}

	if err := sm.Connect("server1"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	// リロード
	reloaded, err := sm.ReloadHosts()
	if err != nil {
		t.Fatalf("ReloadHosts() error = %v", err)
	}

	if len(reloaded) != 2 {
		t.Fatalf("len(hosts) = %d, want 2", len(reloaded))
	}

	// server1 の接続状態が保持されていること
	if reloaded[0].State != core.Connected {
		t.Errorf("server1 state = %v, want %v", reloaded[0].State, core.Connected)
	}
	// server2 は変わらない
	if reloaded[1].State != core.Disconnected {
		t.Errorf("server2 state = %v, want %v", reloaded[1].State, core.Disconnected)
	}
}
