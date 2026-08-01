package agentcontract

import (
	"encoding/json"
	"testing"
)

func TestCodexHumanApprovalPausePersistsRawRequestID(t *testing.T) {
	_, _, pending := codexHumanApprovalPause(
		codexPendingRequestKindCommandApproval,
		"Approve command execution",
		"Command: find .",
		json.RawMessage(`7`),
		"turn-1",
		"item-1",
		json.RawMessage(`{"command":"find ."}`),
	)
	if pending == nil {
		t.Fatal("expected pending request")
	}
	if got := string(pending.RequestIDRaw); got != "7" {
		t.Fatalf("expected raw request id 7, got %q", got)
	}
	if got := pending.RequestID; got != "7" {
		t.Fatalf("expected legacy request id string to be preserved, got %q", got)
	}
}

func TestCodexPendingRequestResponseIDPrefersRawJSONID(t *testing.T) {
	pending := &codexPendingRequest{
		RequestID:    "7",
		RequestIDRaw: json.RawMessage(`7`),
	}
	if got := string(codexPendingRequestResponseID(pending)); got != "7" {
		t.Fatalf("expected numeric JSON id, got %q", got)
	}
}

func TestCodexPendingRequestResponseIDFallsBackToLegacyStringID(t *testing.T) {
	pending := &codexPendingRequest{RequestID: "req-123"}
	if got := string(codexPendingRequestResponseID(pending)); got != `"req-123"` {
		t.Fatalf("expected JSON string id, got %q", got)
	}
}
