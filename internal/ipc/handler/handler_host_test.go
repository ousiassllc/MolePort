package handler

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_HostList(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "host.list", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	hostList, ok := result.(protocol.HostListResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.HostListResult", result)
	}

	if len(hostList.Hosts) != 2 {
		t.Fatalf("hosts count = %d, want 2", len(hostList.Hosts))
	}

	if hostList.Hosts[0].Name != "prod" {
		t.Errorf("hosts[0].Name = %q, want %q", hostList.Hosts[0].Name, "prod")
	}
	if hostList.Hosts[0].State != "connected" {
		t.Errorf("hosts[0].State = %q, want %q", hostList.Hosts[0].State, "connected")
	}
	if hostList.Hosts[1].State != "disconnected" {
		t.Errorf("hosts[1].State = %q, want %q", hostList.Hosts[1].State, "disconnected")
	}
}

func TestHandler_HostReload(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "host.reload", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	reloadResult, ok := result.(protocol.HostReloadResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.HostReloadResult", result)
	}
	if reloadResult.Total != 2 {
		t.Errorf("Total = %d, want 2", reloadResult.Total)
	}
	if len(reloadResult.Added) != 0 {
		t.Errorf("Added = %v, want empty", reloadResult.Added)
	}
	if len(reloadResult.Removed) != 0 {
		t.Errorf("Removed = %v, want empty", reloadResult.Removed)
	}
}

func TestHandler_HostReload_Diff(t *testing.T) {
	h, sshMgr, _, _ := newTestHandler()

	// ReloadHosts 後に "staging" が消え、"dev" が追加される
	sshMgr.reloadHosts = []core.SSHHost{
		{Name: "prod", HostName: "prod.example.com", Port: 22, User: "deploy", State: core.Connected},
		{Name: "dev", HostName: "dev.example.com", Port: 22, User: "deploy", State: core.Disconnected},
	}

	result, rpcErr := h.Handle("client-1", "host.reload", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	reloadResult, ok := result.(protocol.HostReloadResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.HostReloadResult", result)
	}

	if reloadResult.Total != 2 {
		t.Errorf("Total = %d, want 2", reloadResult.Total)
	}
	if len(reloadResult.Added) != 1 || reloadResult.Added[0] != "dev" {
		t.Errorf("Added = %v, want [dev]", reloadResult.Added)
	}
	if len(reloadResult.Removed) != 1 || reloadResult.Removed[0] != "staging" {
		t.Errorf("Removed = %v, want [staging]", reloadResult.Removed)
	}
}
