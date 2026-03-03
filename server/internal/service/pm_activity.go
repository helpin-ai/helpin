package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// PMActivityService contains activity logging business logic.
type PMActivityService struct {
	activityRepo *repository.PMActivityRepository
}

// NewPMActivityService creates a new PMActivityService.
func NewPMActivityService(activityRepo *repository.PMActivityRepository) *PMActivityService {
	return &PMActivityService{activityRepo: activityRepo}
}

// ListEntity returns activity entries for a single entity.
func (s *PMActivityService) ListEntity(ctx context.Context, entityType, entityID string, pagination model.PMPagination) ([]model.ActivityLogEntry, int64, error) {
	if entityType == "" || entityID == "" {
		return nil, 0, fmt.Errorf("entity_type and entity_id are required")
	}
	return s.activityRepo.List(ctx, entityType, entityID, pagination)
}

// ListWorkspace returns workspace feed activity.
func (s *PMActivityService) ListWorkspace(ctx context.Context, workspaceID string, filters model.PMActivityFilters, pagination model.PMPagination) ([]model.ActivityLogEntry, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.activityRepo.ListByWorkspace(ctx, workspaceID, filters, pagination)
}

// Log writes an activity log entry.
func (s *PMActivityService) Log(ctx context.Context, workspaceID, entityType, entityID string, actorID *string, action string, fieldName, oldValue, newValue *string, metadata map[string]interface{}) error {
	if workspaceID == "" || entityType == "" || entityID == "" || action == "" {
		return fmt.Errorf("workspace_id, entity_type, entity_id, and action are required")
	}

	rawMetadata := json.RawMessage(`{}`)
	if metadata != nil {
		bytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal activity metadata: %w", err)
		}
		rawMetadata = bytes
	}

	entry := &model.PMActivityLog{
		WorkspaceID: workspaceID,
		EntityType:  entityType,
		EntityID:    entityID,
		ActorID:     actorID,
		Action:      action,
		FieldName:   fieldName,
		OldValue:    oldValue,
		NewValue:    newValue,
		Metadata:    rawMetadata,
	}
	if err := s.activityRepo.Create(ctx, entry); err != nil {
		return err
	}
	return nil
}
