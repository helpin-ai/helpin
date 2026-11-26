package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
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

func (s *AgentRuntimeProjectionService) SetSlackRunNotifications(n *NotificationService, rules *repository.AutomationRuleRepository) *AgentRuntimeProjectionService {
	s.slackNotifications, s.notificationRules = n, rules
	return s
}

func (s *AgentRuntimeProjectionService) runNotificationRecipient(ctx context.Context, run *model.AgentRun, ruleID string) (string, error) {
	if owner := derefString(run.TriggeredByUserID); owner != "" {
		return owner, nil
	}
	if ruleID == "" || s.notificationRules == nil {
		return "", nil
	}
	rule, err := s.notificationRules.GetByID(ctx, run.WorkspaceID, ruleID)
	if err != nil || rule == nil {
		return "", err
	}
	return derefString(rule.CreatedBy), nil
}

func runNotificationMetadata(run *model.AgentRun) model.JSONB {
	metadata := model.JSONB{"run_id": run.ID, "agent_id": run.AgentID, "target_type": run.TargetType, "target_id": run.TargetID}
	if ruleID := runInputRuleID(run.Input); ruleID != "" {
		metadata["automation_rule_id"] = ruleID
	}
	if run.DockChatID != nil {
		metadata["dock_chat_id"] = *run.DockChatID
	} else if run.TargetType == "task" {
		metadata["task_id"] = run.TargetID
	}
	return metadata
}

func (s *AgentRuntimeProjectionService) notifyRunActivity(ctx context.Context, run *model.AgentRun, kind string) error {
	if s.slackNotifications == nil || run.TargetType == supportPreviewTarget || run.TargetType == model.HelpAssistantTurnTarget {
		return nil
	}
	owner, err := s.runNotificationRecipient(ctx, run, runInputRuleID(run.Input))
	if err != nil || owner == "" {
		return err
	}
	subject := "Your agent run"
	if runInputRuleID(run.Input) != "" {
		subject = "Your workflow run"
	}
	status := run.Status
	if kind == "agent_run.started" {
		status = "started"
	}
	return s.slackNotifications.queueSlackNotification(ctx, owner, model.NotificationEventInput{
		WorkspaceID: run.WorkspaceID, EntityType: "agent_run", EntityID: run.ID,
		EventType: kind, Metadata: runNotificationMetadata(run), Title: subject + " " + status,
	})
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
	if s.attentionNotifier == nil || run.TargetType == supportPreviewTarget || run.TargetType == model.HelpAssistantTurnTarget {
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
	ownerID, err := s.runNotificationRecipient(ctx, run, runInputRuleID(run.Input))
	if err != nil {
		return err
	}
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
	metadata := runNotificationMetadata(run)
	metadata["agent_name"] = name
	metadata["interaction_id"] = firstNonEmptyString(derefString(interaction.RequestID), interaction.ID)
	metadata["interaction_kind"] = interaction.InteractionKind
	event := model.NotificationEventInput{
		WorkspaceID: run.WorkspaceID, EntityType: "agent_run", EntityID: run.ID,
		EventType: taskAgentAttentionRequiredEventType, Category: model.NotifCategoryAgentAttention,
		Title: fmt.Sprintf("%s needs your %s", name, need), Body: derefString(interaction.Title),
		Priority: "high", Metadata: metadata,
		ExplicitRecipients: []string{ownerID}, SkipFollowers: true, SkipEmailDelivery: true,
	}
	if derefString(run.TriggeredByUserID) == "" {
		// Scheduled flows have no manual requester. Only the Slack workflow preference applies to their creator.
		return s.slackNotifications.queueSlackNotification(ctx, ownerID, event)
	}
	return s.attentionNotifier.Emit(ctx, event)
}
