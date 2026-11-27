package handler

import (
	"errors"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_SSHConnect_StartsAutoConnectForwards(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()
	rule := func(name, host string, auto bool) core.ForwardRule {
		return core.ForwardRule{Name: name, Host: host, AutoConnect: auto}
	}
	fwdMgr.rules = []core.ForwardRule{
		rule("auto", "prod", true),
		rule("manual", "prod", false),
		rule("auto-active", "prod", true),
		rule("auto-error", "prod", true),
		rule("auto-other-host", "staging", true),
	}
	fwdMgr.sessions = []core.ForwardSession{
		{Rule: rule("auto", "prod", true), Status: core.Stopped},
		{Rule: rule("manual", "prod", false), Status: core.Stopped},
		{Rule: rule("auto-active", "prod", true), Status: core.Active},
		{Rule: rule("auto-error", "prod", true), Status: core.SessionError},
		{Rule: rule("auto-other-host", "staging", true), Status: core.Stopped},
	}

	params := mustMarshal(t, protocol.SSHConnectParams{Host: "prod"})
	if _, rpcErr := h.Handle("client-1", "ssh.connect", params); rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	want := []string{"auto", "auto-error"}
	if len(fwdMgr.startedRules) != len(want) {
		t.Fatalf("started rules = %v, want %v", fwdMgr.startedRules, want)
	}
	for i, name := range want {
		if fwdMgr.startedRules[i] != name {
			t.Errorf("started rules = %v, want %v", fwdMgr.startedRules, want)
		}
	}
}

func TestHandler_SSHConnect_AutoConnectStartFailureIsNotFatal(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()
	fwdMgr.rules = []core.ForwardRule{{Name: "auto", Host: "prod", AutoConnect: true}}
	fwdMgr.sessions = []core.ForwardSession{{Rule: fwdMgr.rules[0], Status: core.Stopped}}
	fwdMgr.startErr = errors.New("port in use")

	params := mustMarshal(t, protocol.SSHConnectParams{Host: "prod"})
	result, rpcErr := h.Handle("client-1", "ssh.connect", params)
	if rpcErr != nil {
		t.Fatalf("ssh.connect should succeed even if auto-connect start fails: %v", rpcErr)
	}
	if got := result.(protocol.SSHConnectResult).Status; got != protocol.StateConnected {
		t.Errorf("status = %q, want %q", got, protocol.StateConnected)
	}
	if len(fwdMgr.startedRules) != 1 {
		t.Errorf("started rules = %v, want [auto]", fwdMgr.startedRules)
	}
}

func TestHandler_SSHConnect_ConnectFailureSkipsAutoConnect(t *testing.T) {
	h, sshMgr, fwdMgr, _ := newTestHandler()
	fwdMgr.rules = []core.ForwardRule{{Name: "auto", Host: "prod", AutoConnect: true}}
	fwdMgr.sessions = []core.ForwardSession{{Rule: fwdMgr.rules[0], Status: core.Stopped}}
	sshMgr.connectFn = func(string) error { return errors.New("dial failed") }

	params := mustMarshal(t, protocol.SSHConnectParams{Host: "prod"})
	if _, rpcErr := h.Handle("client-1", "ssh.connect", params); rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if len(fwdMgr.startedRules) != 0 {
		t.Errorf("started rules = %v, want none", fwdMgr.startedRules)
	}
}
