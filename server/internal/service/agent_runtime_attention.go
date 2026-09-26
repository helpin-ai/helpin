package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type agentAttentionNotifier interface {
	Emit(context.Context, model.NotificationEventInput) error
	MarkAgentAttentionResolved(context.Context, string, string) error
}

// SetAttentionNotifier enables durable in-app attention alerts for runtime interactions.
func (s *AgentRuntimeProjectionService) SetAttentionNotifier(notifier agentAttentionNotifier) *AgentRuntimeProjectionService {
	s.attentionNotifier = notifier
	return s
}

func interactionNeedsAttention(interaction model.AgentRunInteraction) bool {
	if interaction.Status != model.AgentRunInteractionStatusPending {
		return false
	}
	switch interaction.InteractionKind {
	case model.AgentRunInteractionKindRequestUserInput, model.AgentRunInteractionKindApprovalRequest,
		model.AgentRunInteractionKindReviewCheckpoint, model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval, model.AgentRunInteractionKindPermissionsApproval:
		return true
	default:
		return false
	}
}

func (s *AgentRuntimeProjectionService) syncInteractionAttention(ctx context.Context, run *model.AgentRun, interaction model.AgentRunInteraction) error {
	if s.attentionNotifier == nil || run.TargetType == supportPreviewTarget {
		return nil
	}
	if isTerminalAgentRunStatus(run.Status) {
		return s.attentionNotifier.MarkAgentAttentionResolved(ctx, run.WorkspaceID, run.ID)
	}
	if !interactionNeedsAttention(interaction) {
		interactions, err := s.interactionRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
		if err != nil {
			return err
		}
		for _, other := range interactions {
			if interactionNeedsAttention(other) {
				return nil
			}
		}
		return s.attentionNotifier.MarkAgentAttentionResolved(ctx, run.WorkspaceID, run.ID)
	}
	ownerID := strings.TrimSpace(derefString(run.TriggeredByUserID))
	if ownerID == "" {
		return nil
	}
	name := "Agent"
	if run.DockChatID != nil {
		name = "Ask Agent"
	} else if s.agentRepo != nil {
		agent, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
		if err != nil {
			return err
		}
		if agent != nil && strings.TrimSpace(agent.Name) != "" {
			name = agent.Name
		}
	}
	need := "approval"
	if interaction.InteractionKind == model.AgentRunInteractionKindRequestUserInput {
		need = "input"
	}
	metadata := model.JSONB{
		"run_id": run.ID, "agent_id": run.AgentID, "agent_name": name,
		"interaction_id":   firstNonEmptyString(derefString(interaction.RequestID), interaction.ID),
		"interaction_kind": interaction.InteractionKind,
		"target_type":      run.TargetType, "target_id": run.TargetID,
	}
	if run.DockChatID != nil {
		metadata["dock_chat_id"] = *run.DockChatID
	} else if run.TargetType == "task" {
		metadata["task_id"] = run.TargetID
	}
	return s.attentionNotifier.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: run.WorkspaceID, EntityType: "agent_run", EntityID: run.ID,
		EventType: taskAgentAttentionRequiredEventType, Category: model.NotifCategoryAgentAttention,
		Title: fmt.Sprintf("%s needs your %s", name, need), Body: derefString(interaction.Title),
		Priority: "high", Metadata: metadata,
		ExplicitRecipients: []string{ownerID}, SkipFollowers: true, SkipEmailDelivery: true,
	})
}
