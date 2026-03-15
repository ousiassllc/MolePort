package cli

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestNewCLICredentialHandler_ReturnsNonNil(t *testing.T) {
	handler := newCLICredentialHandler()
	if handler == nil {
		t.Error("newCLICredentialHandler should return a non-nil handler")
	}
}

func TestHandleKeyboardInteractive_EmptyPrompts(t *testing.T) {
	req := protocol.CredentialRequestNotification{
		RequestID: "test-id",
		Type:      "keyboard-interactive",
		Host:      "testhost",
		Prompts:   []protocol.PromptData{},
	}

	resp, err := handleKeyboardInteractive(req)
	if err != nil {
		t.Fatalf("handleKeyboardInteractive with empty prompts: %v", err)
	}
	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if resp.RequestID != "test-id" {
		t.Errorf("RequestID = %q, want %q", resp.RequestID, "test-id")
	}
	if len(resp.Answers) != 0 {
		t.Errorf("Answers = %v, want empty slice", resp.Answers)
	}
}

func TestNewCLICredentialHandler_UnknownType(t *testing.T) {
	handler := newCLICredentialHandler()

	req := protocol.CredentialRequestNotification{
		RequestID: "test-id",
		Type:      "unknown-type",
		Host:      "testhost",
	}

	resp, err := handler(req)
	if err == nil {
		t.Error("unknown credential type should return an error")
	}
	if resp != nil {
		t.Error("response should be nil on error")
	}
}
