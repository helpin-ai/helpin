package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func logTemplateActivity(ctx context.Context, tx *gorm.DB, workspaceID, ruleID, actorID, action string, metadata map[string]any) error {
	rawMetadata := json.RawMessage(`{}`)
	if metadata != nil {
		bytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal template activity metadata: %w", err)
		}
		rawMetadata = bytes
	}

	var actor *string
	if trimmed := strings.TrimSpace(actorID); trimmed != "" {
		actor = &trimmed
	}
	entry := &model.PMActivityLog{
		WorkspaceID: workspaceID,
		EntityType:  "automation_flow",
		EntityID:    ruleID,
		ActorID:     actor,
		Action:      action,
		Metadata:    rawMetadata,
	}
	if err := tx.WithContext(ctx).Create(entry).Error; err != nil {
		return fmt.Errorf("log template activity: %w", err)
	}
	return nil
}

func agentIDForActivity(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	return strings.TrimSpace(agent.ID)
}
