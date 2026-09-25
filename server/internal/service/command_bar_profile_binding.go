package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type commandBarAIContextKey struct{}

// commandBarAIContext marks a DAG dependency as context, not an AI continuation.
// An explicit epic selection is loaded only from the trusted plan row.
func (s *AgentService) commandBarAIContext(ctx context.Context, workspaceID, planID, actorID string) (context.Context, string, error) {
	var selection *model.AIExecutionSelection
	if s.commandBarPlanRepo != nil && strings.TrimSpace(planID) != "" {
		plan, err := s.commandBarPlanRepo.GetByID(ctx, workspaceID, planID)
		if err != nil {
			return ctx, actorID, err
		}
		if plan == nil {
			return ctx, actorID, fmt.Errorf("command bar plan not found")
		}
		if len(plan.ProfileBinding) > 0 && string(plan.ProfileBinding) != "null" {
			if err := json.Unmarshal(plan.ProfileBinding, &selection); err != nil {
				return ctx, actorID, fmt.Errorf("decode delivery profile binding: %w", err)
			}
		}
	}
	if selection != nil {
		if owner, personal := selection.PersonalOwner(); personal {
			if owner == "" {
				return ctx, actorID, fmt.Errorf("personal delivery profile has no owner")
			}
			actorID = owner
		}
	}
	return context.WithValue(ctx, commandBarAIContextKey{}, selection), actorID, nil
}

func requireCommandBarPlanProfileOwner(plan model.CommandBarPlanRecord, actorID string) error {
	if len(plan.ProfileBinding) == 0 {
		return nil
	}
	var selection model.AIExecutionSelection
	if err := json.Unmarshal(plan.ProfileBinding, &selection); err != nil {
		return err
	}
	if owner, personal := selection.PersonalOwner(); personal && owner != strings.TrimSpace(actorID) {
		return fmt.Errorf("this delivery uses another member's personal profile; restart the affected step with your profile or agent defaults")
	}
	return nil
}
