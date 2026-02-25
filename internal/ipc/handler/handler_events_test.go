package handler

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestHandler_EventsSubscribe(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.EventsSubscribeParams{Types: []string{"ssh", "forward"}})
	result, rpcErr := h.Handle("client-1", "events.subscribe", params)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	subResult, ok := result.(protocol.EventsSubscribeResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.EventsSubscribeResult", result)
	}
	if subResult.SubscriptionID == "" {
		t.Error("SubscriptionID should not be empty")
	}
}

func TestHandler_EventsUnsubscribe(t *testing.T) {
	h, _, _, _ := newTestHandler()

	// まず購読を作成
	subParams := mustMarshal(t, protocol.EventsSubscribeParams{Types: []string{"ssh"}})
	subResult, _ := h.Handle("client-1", "events.subscribe", subParams)
	subID := subResult.(protocol.EventsSubscribeResult).SubscriptionID

	// 購読を解除
	unsubParams := mustMarshal(t, protocol.EventsUnsubscribeParams{SubscriptionID: subID})
	result, rpcErr := h.Handle("client-1", "events.unsubscribe", unsubParams)
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}

	unsubResult, ok := result.(protocol.EventsUnsubscribeResult)
	if !ok {
		t.Fatalf("result type = %T, want protocol.EventsUnsubscribeResult", result)
	}
	if !unsubResult.OK {
		t.Error("OK should be true")
	}

	// 存在しない購読 ID で解除するとエラー
	badParams := mustMarshal(t, protocol.EventsUnsubscribeParams{SubscriptionID: "nonexistent"})
	_, rpcErr = h.Handle("client-1", "events.unsubscribe", badParams)
	if rpcErr == nil {
		t.Fatal("expected RPC error for nonexistent subscription")
	}
}

func TestHandler_EventsSubscribe_InvalidType(t *testing.T) {
	h, _, _, _ := newTestHandler()

	params := mustMarshal(t, protocol.EventsSubscribeParams{Types: []string{"ssh", "invalid"}})
	_, rpcErr := h.Handle("client-1", "events.subscribe", params)
	if rpcErr == nil {
		t.Fatal("expected RPC error for invalid event type")
	}
	if rpcErr.Code != protocol.InvalidParams {
		t.Errorf("error code = %d, want %d (InvalidParams)", rpcErr.Code, protocol.InvalidParams)
	}
}
