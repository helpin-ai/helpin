package model

import "time"

const (
	AgentRunArtifactTypeProviderResponseCheckpoint = "provider_response_checkpoint"
)

type ProviderResponseCheckpoint struct {
	Provider               string    `json:"provider"`
	ResponseID             string    `json:"response_id"`
	PreviousResponseID     string    `json:"previous_response_id,omitempty"`
	AssistantMessageSeqNo  int       `json:"assistant_message_seq_no"`
	RecordedAt             time.Time `json:"recorded_at,omitempty"`
}
