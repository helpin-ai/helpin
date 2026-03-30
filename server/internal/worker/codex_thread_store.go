package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const codexSessionStateArtifactType = "codex_session_state"

const (
	codexPendingRequestKindHumanInput      = "human_input"
	codexPendingRequestKindCommandApproval = "command_execution"
	codexPendingRequestKindFileApproval    = "file_change"
	codexPendingRequestKindPermissions     = "permissions"
)

type codexSessionState struct {
	ThreadID                  string               `json:"thread_id,omitempty"`
	ThreadPath                string               `json:"thread_path,omitempty"`
	HomeRoot                  string               `json:"home_root,omitempty"`
	CodexHome                 string               `json:"codex_home,omitempty"`
	Provider                  string               `json:"provider,omitempty"`
	Model                     string               `json:"model,omitempty"`
	AuthMode                  string               `json:"auth_mode,omitempty"`
	Sandbox                   string               `json:"sandbox,omitempty"`
	InvocationMode            string               `json:"invocation_mode,omitempty"`
	LastSubmittedMessageSeqNo int                  `json:"last_submitted_message_sequence_no,omitempty"`
	PendingRequest            *codexPendingRequest `json:"pending_request,omitempty"`
	ClearedAt                 *time.Time           `json:"cleared_at,omitempty"`
}

type codexPendingRequest struct {
	Kind         string          `json:"kind,omitempty"`
	RequestID    string          `json:"request_id,omitempty"`
	RequestIDRaw json.RawMessage `json:"request_id_raw,omitempty"`
	TurnID       string          `json:"turn_id,omitempty"`
	ItemID       string          `json:"item_id,omitempty"`
	QuestionIDs  []string        `json:"question_ids,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
}

type codexThreadStore struct {
	artifactRepo *repository.AgentRunArtifactRepository
}

func newCodexThreadStore(artifactRepo *repository.AgentRunArtifactRepository) *codexThreadStore {
	return &codexThreadStore{artifactRepo: artifactRepo}
}

func (s *codexThreadStore) Load(ctx context.Context, run *model.AgentRun) (*codexSessionState, error) {
	if s == nil || s.artifactRepo == nil || run == nil {
		return nil, nil
	}
	artifacts, err := s.artifactRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != codexSessionStateArtifactType || artifact.InlineContent == nil {
			continue
		}
		var state codexSessionState
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &state); err != nil {
			return nil, fmt.Errorf("parse codex session state: %w", err)
		}
		if state.ClearedAt != nil {
			return nil, nil
		}
		return &state, nil
	}
	return nil, nil
}

func (s *codexThreadStore) Save(ctx context.Context, run *model.AgentRun, state *codexSessionState) error {
	if s == nil || s.artifactRepo == nil || run == nil || state == nil {
		return nil
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal codex session state: %w", err)
	}
	content := string(payload)
	seqNo, err := s.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	return s.artifactRepo.Create(ctx, &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  codexSessionStateArtifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata: json.RawMessage(`{
			"internal": true
		}`),
		SequenceNo: seqNo,
	})
}

func (s *codexThreadStore) Clear(ctx context.Context, run *model.AgentRun) error {
	state := &codexSessionState{
		ClearedAt: timePtr(time.Now().UTC()),
	}
	return s.Save(ctx, run, state)
}

func timePtr(value time.Time) *time.Time {
	return &value
}
