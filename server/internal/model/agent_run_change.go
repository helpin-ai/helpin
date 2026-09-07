package model

// AgentRunChangeKind distinguishes persisted content from a run state snapshot.
// An omitted kind remains a conservative, unspecified update for older producers.
type AgentRunChangeKind string

const (
	AgentRunChangeState       AgentRunChangeKind = "state"
	AgentRunChangeMessage     AgentRunChangeKind = "message"
	AgentRunChangeInteraction AgentRunChangeKind = "interaction"
	AgentRunChangeArtifact    AgentRunChangeKind = "artifact"
)
