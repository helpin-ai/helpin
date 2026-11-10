package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrSupportRequestTypeNotFound means the request type does not exist in the
// workspace.
var ErrSupportRequestTypeNotFound = errors.New("request type not found")

// ErrSupportRequestTypeNameTaken means another request type has the name.
var ErrSupportRequestTypeNameTaken = errors.New("a request type with this name already exists")

// ErrSupportRequestTypeInUse means conversations carry the request type, so
// it can be turned off but not deleted.
var ErrSupportRequestTypeInUse = errors.New("conversations use this request type; turn it off instead")

// SupportRequestTypeRepository stores request types.
type SupportRequestTypeRepository struct {
	db *gorm.DB
}

// NewSupportRequestTypeRepository creates the repository.
func NewSupportRequestTypeRepository(db *gorm.DB) *SupportRequestTypeRepository {
	return &SupportRequestTypeRepository{db: db}
}

// List returns the workspace's request types in display order; enabledOnly
// leaves out disabled ones.
func (r *SupportRequestTypeRepository) List(ctx context.Context, workspaceID string, enabledOnly bool) ([]model.SupportRequestType, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	var types []model.SupportRequestType
	if err := query.Order("position, created_at, id").Find(&types).Error; err != nil {
		return nil, fmt.Errorf("list request types: %w", err)
	}
	return types, nil
}

// CountConversations returns how many conversations carry each type.
func (r *SupportRequestTypeRepository) CountConversations(ctx context.Context, workspaceID string) (map[string]int64, error) {
	var rows []struct {
		RequestTypeID string
		Count         int64
	}
	if err := r.db.WithContext(ctx).Table("support_conversations").
		Select("request_type_id, COUNT(*) AS count").
		Where("workspace_id = ? AND request_type_id IS NOT NULL", workspaceID).
		Group("request_type_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count conversations per request type: %w", err)
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.RequestTypeID] = row.Count
	}
	return counts, nil
}

// Get returns one request type.
func (r *SupportRequestTypeRepository) Get(ctx context.Context, workspaceID, id string) (*model.SupportRequestType, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrSupportRequestTypeNotFound
	}
	var requestType model.SupportRequestType
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Take(&requestType).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupportRequestTypeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get request type: %w", err)
	}
	return &requestType, nil
}

// Count returns how many request types the workspace has.
func (r *SupportRequestTypeRepository) Count(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.SupportRequestType{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count request types: %w", err)
	}
	return count, nil
}

// NameTaken reports whether another request type of the workspace has the
// name, ignoring case.
func (r *SupportRequestTypeRepository) NameTaken(ctx context.Context, workspaceID, name, exceptID string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&model.SupportRequestType{}).Where("workspace_id = ? AND lower(name) = lower(?)", workspaceID, name)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("check request type name: %w", err)
	}
	return count > 0, nil
}

// Create appends a request type after the workspace's existing ones.
func (r *SupportRequestTypeRepository) Create(ctx context.Context, requestType *model.SupportRequestType) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var last struct{ Position *int }
		if err := tx.Model(&model.SupportRequestType{}).Select("MAX(position) AS position").
			Where("workspace_id = ?", requestType.WorkspaceID).Scan(&last).Error; err != nil {
			return fmt.Errorf("find last request type position: %w", err)
		}
		requestType.Position = 0
		if last.Position != nil {
			requestType.Position = *last.Position + 1
		}
		if requestType.ID == "" {
			requestType.ID = uuid.NewString()
		}
		if err := tx.Create(requestType).Error; err != nil {
			if isDuplicateKeyError(err) {
				return ErrSupportRequestTypeNameTaken
			}
			return fmt.Errorf("create request type: %w", err)
		}
		return nil
	})
}

// Update saves every editable field of a request type.
func (r *SupportRequestTypeRepository) Update(ctx context.Context, requestType *model.SupportRequestType) error {
	result := r.db.WithContext(ctx).Model(&model.SupportRequestType{}).
		Where("workspace_id = ? AND id = ?", requestType.WorkspaceID, requestType.ID).
		Updates(map[string]any{
			"name": requestType.Name, "description": requestType.Description, "fields": requestType.Fields,
			"default_priority": requestType.DefaultPriority, "mailbox_id": requestType.MailboxID,
			"enabled": requestType.Enabled, "updated_at": time.Now().UTC(),
		})
	if result.Error != nil && isDuplicateKeyError(result.Error) {
		return ErrSupportRequestTypeNameTaken
	}
	if result.Error != nil {
		return fmt.Errorf("update request type: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSupportRequestTypeNotFound
	}
	return nil
}

// Delete removes a request type no conversation carries and clears it from
// SLA policy conditions; a policy that matched only this type is turned off
// so it does not start matching every conversation.
func (r *SupportRequestTypeRepository) Delete(ctx context.Context, workspaceID, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var used int64
		if err := tx.Table("support_conversations").Where("workspace_id = ? AND request_type_id = ?", workspaceID, id).
			Limit(1).Count(&used).Error; err != nil {
			return fmt.Errorf("check request type use: %w", err)
		}
		if used > 0 {
			return ErrSupportRequestTypeInUse
		}
		result := tx.Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.SupportRequestType{})
		if result.Error != nil {
			return fmt.Errorf("delete request type: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrSupportRequestTypeNotFound
		}
		var policies []model.SupportSLAPolicy
		if err := tx.Where("workspace_id = ?", workspaceID).Find(&policies).Error; err != nil {
			return fmt.Errorf("list SLA policies: %w", err)
		}
		for _, policy := range policies {
			if !slices.Contains([]string(policy.RequestTypeIDs), id) {
				continue
			}
			remaining := model.DocsStringArray(slices.DeleteFunc(slices.Clone([]string(policy.RequestTypeIDs)), func(typeID string) bool { return typeID == id }))
			updates := map[string]any{"request_type_ids": remaining, "updated_at": time.Now().UTC()}
			if len(remaining) == 0 {
				updates["enabled"] = false
			}
			if err := tx.Model(&model.SupportSLAPolicy{}).Where("workspace_id = ? AND id = ?", workspaceID, policy.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("remove request type from SLA policy: %w", err)
			}
		}
		return nil
	})
}

// Reorder sets positions to the order given; the list must hold every
// request type of the workspace exactly once.
func (r *SupportRequestTypeRepository) Reorder(ctx context.Context, workspaceID string, ids []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []string
		if err := tx.Model(&model.SupportRequestType{}).Where("workspace_id = ?", workspaceID).Pluck("id", &existing).Error; err != nil {
			return fmt.Errorf("list request types to reorder: %w", err)
		}
		if len(existing) != len(ids) {
			return ErrSupportRequestTypeNotFound
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if !slices.Contains(existing, id) || seen[id] {
				return ErrSupportRequestTypeNotFound
			}
			seen[id] = true
		}
		for position, id := range ids {
			if err := tx.Model(&model.SupportRequestType{}).Where("workspace_id = ? AND id = ?", workspaceID, id).
				Updates(map[string]any{"position": position, "updated_at": time.Now().UTC()}).Error; err != nil {
				return fmt.Errorf("reorder request types: %w", err)
			}
		}
		return nil
	})
}
