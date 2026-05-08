package model

import (
	"encoding/json"
	"strings"
	"time"
)

type CodingSession struct {
	ID                  string                       `json:"id"`
	RunID               string                       `json:"run_id"`
	ParentRunID         *string                      `json:"parent_run_id,omitempty"`
	WorkspaceID         string                       `json:"workspace_id"`
	TargetType          string                       `json:"target_type"`
	TargetID            string                       `json:"target_id"`
	AgentID             string                       `json:"agent_id"`
	RuntimeKind         string                       `json:"runtime_kind"`
	InvocationMode      string                       `json:"invocation_mode"`
	Status              string                       `json:"status"`
	PauseReason         string                       `json:"pause_reason"`
	ErrorMessage        *string                      `json:"error_message,omitempty"`
	ExecutionStage      *string                      `json:"execution_stage,omitempty"`
	LastHeartbeatAt     *time.Time                   `json:"last_heartbeat_at,omitempty"`
	StartedAt           *time.Time                   `json:"started_at,omitempty"`
	Title               string                       `json:"title"`
	Summary             *string                      `json:"summary,omitempty"`
	SystemPrompt        *string                      `json:"system_prompt,omitempty"`
	Capabilities        CodingSessionCapabilities    `json:"capabilities"`
	Repo                CodingSessionRepoState       `json:"repo"`
	CachedInputTokens   int                          `json:"cached_input_tokens"`
	InputTokens         int                          `json:"input_tokens"`
	OutputTokens        int                          `json:"output_tokens"`
	TokensUsed          int                          `json:"tokens_used"`
	AuthState           *CodexAuthState              `json:"auth_state,omitempty"`
	StreamStateSnapshot *CodingSessionStreamSnapshot `json:"stream_state_snapshot,omitempty"`
	TriggeredByUser     *CodingSessionActor          `json:"triggered_by_user,omitempty"`
	CreatedAt           time.Time                    `json:"created_at"`
	UpdatedAt           time.Time                    `json:"updated_at"`
}

// CodingSessionActor describes the human user who triggered a coding session run.
type CodingSessionActor struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FullName  string  `json:"full_name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type CodingSessionCapabilities struct {
	LiveTextStreaming bool `json:"live_text_streaming"`
	ToolStreaming     bool `json:"tool_streaming"`
	RepoDiffStreaming bool `json:"repo_diff_streaming"`
	PlanStreaming     bool `json:"plan_streaming"`
	Approvals         bool `json:"approvals"`
	HumanInput        bool `json:"human_input"`
	Authentication    bool `json:"authentication"`
	Previews          bool `json:"previews"`
	TerminalOutput    bool `json:"terminal_output"`
	Checkpoints       bool `json:"checkpoints"`
}

type CodingSessionRepoState struct {
	RepoName         *string                 `json:"repo_name,omitempty"`
	Branch           *string                 `json:"branch,omitempty"`
	IsDirty          bool                    `json:"is_dirty"`
	ChangedFileCount int                     `json:"changed_file_count"`
	ChangedFiles     []CodingSessionRepoFile `json:"changed_files,omitempty"`
}

type CodingSessionRepoFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type CodingSessionDiff struct {
	Path        *string `json:"path,omitempty"`
	Diff        string  `json:"diff"`
	IsTruncated bool    `json:"is_truncated"`
}

type CodingSessionEvent struct {
	ID              string         `json:"id"`
	SessionID       string         `json:"session_id"`
	RunID           string         `json:"run_id"`
	SequenceNo      int            `json:"sequence_no"`
	Timestamp       time.Time      `json:"timestamp"`
	Type            string         `json:"type"`
	RuntimeKind     string         `json:"runtime_kind"`
	Payload         map[string]any `json:"payload"`
	RuntimeMetadata map[string]any `json:"runtime_metadata,omitempty"`
}

type CodingSessionEventListResponse struct {
	Events         []CodingSessionEvent `json:"events"`
	NextSequenceNo int                  `json:"next_sequence_no"`
}

func CodingSessionEventFromAgentRunMessage(run *AgentRun, message *AgentRunMessage) CodingSessionEvent {
	if run == nil || message == nil {
		return CodingSessionEvent{}
	}

	eventType := "user.message.completed"
	switch strings.TrimSpace(message.Role) {
	case "assistant":
		eventType = "assistant.message.completed"
	case "tool":
		eventType = "tool.call.completed"
	}

	return CodingSessionEvent{
		ID:          "msg:" + message.ID,
		SessionID:   run.ID,
		RunID:       run.ID,
		SequenceNo:  message.SequenceNo,
		Timestamp:   message.CreatedAt.UTC(),
		Type:        eventType,
		RuntimeKind: run.RuntimeKind,
		Payload: map[string]any{
			"message_id":       message.ID,
			"role":             message.Role,
			"message_type":     message.MessageType,
			"content":          message.Content,
			"sequence_no":      message.SequenceNo,
			"content_blocks":   json.RawMessage(message.ContentBlocks),
			"turn_segments":    json.RawMessage(message.TurnSegments),
			"tool_invocations": json.RawMessage(message.ToolInvocations),
		},
		RuntimeMetadata: map[string]any{
			"source": "agent_run_message",
		},
	}
}

type CodingSessionLiveToolResult struct {
	MessageID     string  `json:"message_id,omitempty"`
	Content       string  `json:"content"`
	OutputSummary *string `json:"output_summary,omitempty"`
	Error         *string `json:"error,omitempty"`
}

type CodingSessionLiveToolCall struct {
	ToolCallID      string                       `json:"tool_call_id"`
	ParentMessageID string                       `json:"parent_message_id,omitempty"`
	ToolName        string                       `json:"tool_name"`
	ArgsText        string                       `json:"args_text"`
	Status          string                       `json:"status"`
	DurationMs      *int64                       `json:"duration_ms,omitempty"`
	StartedAt       *time.Time                   `json:"started_at,omitempty"`
	CompletedAt     *time.Time                   `json:"completed_at,omitempty"`
	Result          *CodingSessionLiveToolResult `json:"result,omitempty"`
}

type CodingSessionLiveAssistantMessage struct {
	MessageID   string                      `json:"message_id"`
	Content     string                      `json:"content"`
	StartedAt   *time.Time                  `json:"started_at,omitempty"`
	CompletedAt *time.Time                  `json:"completed_at,omitempty"`
	Status      string                      `json:"status"`
	ToolCalls   []CodingSessionLiveToolCall `json:"tool_calls,omitempty"`
}

type CodingSessionLiveReasoningMessage struct {
	MessageID      string     `json:"message_id"`
	Content        string     `json:"content"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Status         string     `json:"status"`
	EncryptedValue *string    `json:"encrypted_value,omitempty"`
}

type CodingSessionLiveTurnSegment struct {
	SegmentID        string                             `json:"segment_id"`
	Kind             string                             `json:"kind"`
	AssistantMessage *CodingSessionLiveAssistantMessage `json:"assistant_message,omitempty"`
	ToolCall         *CodingSessionLiveToolCall         `json:"tool_call,omitempty"`
}

type CodingSessionRunPlanStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
}

type CodingSessionRunPlan struct {
	Note string                     `json:"note,omitempty"`
	Plan []CodingSessionRunPlanStep `json:"plan"`
}

type CodingSessionStreamSnapshot struct {
	LiveAssistantMessage *CodingSessionLiveAssistantMessage `json:"live_assistant_message,omitempty"`
	LiveReasoningMessage *CodingSessionLiveReasoningMessage `json:"live_reasoning_message,omitempty"`
	LiveTurnSegments     []CodingSessionLiveTurnSegment     `json:"live_turn_segments,omitempty"`
	CurrentPlan          *CodingSessionRunPlan              `json:"current_plan,omitempty"`
}
