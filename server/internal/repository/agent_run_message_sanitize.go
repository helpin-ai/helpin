package repository

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const postgresJSONReplacement = "\uFFFD"

func sanitizeAgentRunMessageForPostgres(message *model.AgentRunMessage) {
	if message == nil {
		return
	}

	message.Content = sanitizePostgresJSONString(message.Content)
	message.ContentBlocks = sanitizePostgresJSONRawMessage(message.ContentBlocks, nil)
	message.TurnSegments = sanitizePostgresJSONRawMessage(message.TurnSegments, nil)
	message.ToolInvocations = sanitizePostgresJSONRawMessage(message.ToolInvocations, nil)
	message.TokenUsage = sanitizePostgresJSONRawMessage(message.TokenUsage, nil)
}

func sanitizeAgentRunArtifactForPostgres(artifact *model.AgentRunArtifact) {
	if artifact == nil {
		return
	}
	if artifact.InlineContent != nil {
		content := sanitizePostgresJSONString(*artifact.InlineContent)
		artifact.InlineContent = &content
	}
	artifact.Metadata = sanitizePostgresJSONRawMessage(artifact.Metadata, json.RawMessage(`{}`))
}

func sanitizeCodingSessionStateSnapshotForPostgres(snapshot *model.CodingSessionStateSnapshot) {
	if snapshot == nil {
		return
	}
	snapshot.SnapshotPayload = sanitizePostgresJSONRawMessage(snapshot.SnapshotPayload, json.RawMessage(`{}`))
}

func sanitizePostgresJSONString(value string) string {
	if value == "" {
		return value
	}

	value = strings.ToValidUTF8(value, postgresJSONReplacement)
	if !strings.ContainsRune(value, '\x00') {
		return value
	}
	return strings.ReplaceAll(value, "\x00", postgresJSONReplacement)
}

func sanitizePostgresJSONRawMessage(raw json.RawMessage, fallback json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return fallback
	}

	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fallback
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fallback
	}

	sanitized, err := json.Marshal(sanitizePostgresJSONValue(value))
	if err != nil {
		return fallback
	}
	return sanitized
}

func sanitizePostgresJSONValue(value any) any {
	switch typed := value.(type) {
	case string:
		return sanitizePostgresJSONString(typed)
	case []any:
		for i := range typed {
			typed[i] = sanitizePostgresJSONValue(typed[i])
		}
		return typed
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, value := range typed {
			sanitized[sanitizePostgresJSONString(key)] = sanitizePostgresJSONValue(value)
		}
		return sanitized
	default:
		return typed
	}
}
