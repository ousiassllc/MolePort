package handler

import (
	"encoding/json"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// validEventTypes は有効なイベント種別。
var validEventTypes = map[string]bool{
	"ssh":     true,
	"forward": true,
	"metrics": true,
}

func (h *Handler) eventsSubscribe(clientID string, params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.EventsSubscribeParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	for _, t := range p.Types {
		if !validEventTypes[t] {
			return nil, &protocol.RPCError{Code: protocol.InvalidParams, Message: "invalid event type: " + t}
		}
	}

	subID := h.broker.Subscribe(clientID, p.Types)
	return protocol.EventsSubscribeResult{SubscriptionID: subID}, nil
}

func (h *Handler) eventsUnsubscribe(params json.RawMessage) (any, *protocol.RPCError) {
	var p protocol.EventsUnsubscribeParams
	if err := parseParams(params, &p); err != nil {
		return nil, err
	}

	if !h.broker.Unsubscribe(p.SubscriptionID) {
		return nil, &protocol.RPCError{Code: protocol.InvalidParams, Message: "subscription not found"}
	}

	return protocol.EventsUnsubscribeResult{OK: true}, nil
}
