package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSanitizeAgentRunMessageForPostgresRemovesNullCharacters(t *testing.T) {
	message := &model.AgentRunMessage{
		Content:         "created task\x00with scanner output",
		ContentBlocks:   json.RawMessage(`[{"type":"tool_result","text":"secret\u0000match"}]`),
		TurnSegments:    json.RawMessage(`[{"kind":"assistant_message","assistant_message":{"content":"triage\u0000done"}}]`),
		ToolInvocations: json.RawMessage(`[{"tool_name":"scan_gitleaks","result":{"bad\u0000key":"finding\u0000value"}}]`),
		TokenUsage:      json.RawMessage(`{"input_tokens":1,"note":"usage\u0000metadata"}`),
	}

	sanitizeAgentRunMessageForPostgres(message)

	if strings.ContainsRune(message.Content, '\x00') {
		t.Fatalf("content still contains null character: %q", message.Content)
	}

	for name, raw := range map[string]json.RawMessage{
		"content_blocks":   message.ContentBlocks,
		"turn_segments":    message.TurnSegments,
		"tool_invocations": message.ToolInvocations,
		"token_usage":      message.TokenUsage,
	} {
		if strings.Contains(string(raw), `\u0000`) {
			t.Fatalf("%s still contains postgres-rejected null escape: %s", name, string(raw))
		}
		if strings.ContainsRune(string(raw), '\x00') {
			t.Fatalf("%s still contains raw null character: %s", name, string(raw))
		}
		if !json.Valid(raw) {
			t.Fatalf("%s is not valid json after sanitization: %s", name, string(raw))
		}
	}
}

func TestSanitizePostgresJSONRawMessageDropsInvalidJSON(t *testing.T) {
	raw := sanitizePostgresJSONRawMessage(json.RawMessage(`{"unterminated"`))
	if raw != nil {
		t.Fatalf("expected invalid json to be dropped, got %s", string(raw))
	}
}
