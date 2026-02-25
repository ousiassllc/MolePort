package handler

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_ForwardList(t *testing.T) {
	h, _, _, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "forward.list", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	fwdList, ok := result.(protocol.ForwardListResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ForwardListResult", result)
	}

	if len(fwdList.Forwards) != 1 {
		t.Fatalf("forwards count = %d, want 1", len(fwdList.Forwards))
	}
	if fwdList.Forwards[0].Name != "web" {
		t.Errorf("forwards[0].Name = %q, want %q", fwdList.Forwards[0].Name, "web")
	}
	if fwdList.Forwards[0].Type != "local" {
		t.Errorf("forwards[0].Type = %q, want %q", fwdList.Forwards[0].Type, "local")
	}
}

func TestHandler_ForwardAdd_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.ForwardAddParams{
		Name:       "db-tunnel",
		Host:       "prod",
		Type:       "local",
		LocalPort:  5432,
		RemoteHost: "localhost",
		RemotePort: 5432,
	})

	result, rpcErr := h.Handle("client-1", "forward.add", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	addResult, ok := result.(protocol.ForwardAddResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ForwardAddResult", result)
	}
	if addResult.Name != "db-tunnel" {
		t.Errorf("name = %q, want %q", addResult.Name, "db-tunnel")
	}
}

func TestHandler_ForwardAdd_RuleAlreadyExists(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()
	fwdMgr.addErr = fmt.Errorf("rule %q already exists", "web")

	params := mustMarshal(t, protocol.ForwardAddParams{
		Name:       "web",
		Host:       "prod",
		Type:       "local",
		LocalPort:  8080,
		RemoteHost: "localhost",
		RemotePort: 80,
	})

	_, rpcErr := h.Handle("client-1", "forward.add", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != protocol.RuleAlreadyExists {
		t.Errorf("error code = %d, want %d (RuleAlreadyExists)", rpcErr.Code, protocol.RuleAlreadyExists)
	}
}

func TestHandler_ForwardDelete_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.ForwardDeleteParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "forward.delete", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	deleteResult, ok := result.(protocol.ForwardDeleteResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ForwardDeleteResult", result)
	}
	if !deleteResult.OK {
		t.Error("OK should be true")
	}
}

func TestHandler_ForwardDelete_NotFound(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()
	fwdMgr.deleteErr = fmt.Errorf("rule %q not found", "nonexistent")

	params := mustMarshal(t, protocol.ForwardDeleteParams{Name: "nonexistent"})
	_, rpcErr := h.Handle("client-1", "forward.delete", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error")
	}
	if rpcErr.Code != protocol.RuleNotFound {
		t.Errorf("error code = %d, want %d (RuleNotFound)", rpcErr.Code, protocol.RuleNotFound)
	}
}

func TestHandler_ForwardStart_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.ForwardStartParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "forward.start", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	startResult, ok := result.(protocol.ForwardStartResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ForwardStartResult", result)
	}
	if startResult.Status != "active" {
		t.Errorf("status = %q, want %q", startResult.Status, "active")
	}
}

func TestHandler_ForwardStop_Success(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.ForwardStopParams{Name: "web"})
	result, rpcErr := h.Handle("client-1", "forward.stop", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	stopResult, ok := result.(protocol.ForwardStopResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ForwardStopResult", result)
	}
	if stopResult.Status != "stopped" {
		t.Errorf("status = %q, want %q", stopResult.Status, "stopped")
	}
}

func TestHandler_ForwardStopAll(t *testing.T) {
	h, _, fwdMgr, _ := newTestHandler()

	result, rpcErr := h.Handle("client-1", "forward.stopAll", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	stopAllResult, ok := result.(protocol.ForwardStopAllResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.ForwardStopAllResult", result)
	}

	// sessions には Active が 1 件ある
	if stopAllResult.Stopped != 1 {
		t.Errorf("Stopped = %d, want 1", stopAllResult.Stopped)
	}
	if !fwdMgr.stopAllCalled {
		t.Error("StopAllForwards should have been called")
	}
}

func TestHandler_ForwardAdd_SavesConfig(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	params := mustMarshal(t, protocol.ForwardAddParams{
		Name:       "db-tunnel",
		Host:       "prod",
		Type:       "local",
		LocalPort:  5432,
		RemoteHost: "localhost",
		RemotePort: 5432,
	})

	before := cfgMgr.updateCallCount
	_, rpcErr := h.Handle("client-1", "forward.add", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	if cfgMgr.updateCallCount <= before {
		t.Error("forward.add should call UpdateConfig to auto-save rules")
	}

	// 保存されたルールに追加したルールが含まれることを確認
	cfg := cfgMgr.GetConfig()
	found := false
	for _, f := range cfg.Forwards {
		if f.Name == "db-tunnel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("config.Forwards should contain the added rule")
	}
}

func TestHandler_ForwardDelete_SavesConfig(t *testing.T) {
	h, _, _, cfgMgr := newTestHandler()

	params := mustMarshal(t, protocol.ForwardDeleteParams{Name: "web"})

	before := cfgMgr.updateCallCount
	_, rpcErr := h.Handle("client-1", "forward.delete", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	if cfgMgr.updateCallCount <= before {
		t.Error("forward.delete should call UpdateConfig to auto-save rules")
	}
}

func TestHandler_ForwardStart_PreConnectsWithCallback(t *testing.T) {
	h, sshMgr, fwdMgr, _ := newTestHandler()
	sender := &mockNotificationSender{}
	h.SetSender(sender)

	// ホストは未接続
	sshMgr.connected = map[string]bool{"prod": false}

	// ConnectWithCallback が呼ばれたことを記録
	var cbWasNonNil bool
	sshMgr.connectWithCbFn = func(hostName string, cb core.CredentialCallback) error {
		cbWasNonNil = cb != nil
		// 接続成功をシミュレート
		sshMgr.connected[hostName] = true
		return nil
	}

	// セッション情報にルールを追加（GetSession で Host を取得するため）
	fwdMgr.sessions = append(fwdMgr.sessions, core.ForwardSession{
		Rule:   core.ForwardRule{Name: "db", Host: "prod", Type: core.Local, LocalPort: 5432, RemoteHost: "localhost", RemotePort: 5432},
		Status: core.Stopped,
	})

	params := mustMarshal(t, protocol.ForwardStartParams{Name: "db"})
	_, rpcErr := h.Handle("client-1", "forward.start", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	if !cbWasNonNil {
		t.Error("forwardStart should call ConnectWithCallback with non-nil callback when sender is set")
	}
}
