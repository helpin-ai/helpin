package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMListRepository handles DB operations for CRM lists.
type CRMListRepository struct {
	db *gorm.DB
}

// NewCRMListRepository creates a new CRMListRepository.
func NewCRMListRepository(db *gorm.DB) *CRMListRepository {
	return &CRMListRepository{db: db}
}

// List returns lists for a workspace with optional filters.
func (r *CRMListRepository) List(ctx context.Context, workspaceID string, filters model.CRMListFilters, pagination model.PMPagination) ([]model.CRMList, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMList{}).Where("workspace_id = ?", workspaceID)

	if filters.ObjectType != nil && *filters.ObjectType != "" {
		query = query.Where("object_type = ?", *filters.ObjectType)
	}
	if filters.ListType != nil && *filters.ListType != "" {
		query = query.Where("list_type = ?", *filters.ListType)
	}
	if filters.Search != nil && *filters.Search != "" {
		search := "%" + *filters.Search + "%"
		query = query.Where("name ILIKE ?", search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count lists: %w", err)
	}

	var lists []model.CRMList
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&lists).Error; err != nil {
		return nil, 0, fmt.Errorf("list lists: %w", err)
	}
	return lists, total, nil
}

// GetByID returns a list by ID.
func (r *CRMListRepository) GetByID(ctx context.Context, id string) (*model.CRMList, error) {
	var list model.CRMList
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&list).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get list: %w", err)
	}
	return &list, nil
}

// Create inserts a list.
func (r *CRMListRepository) Create(ctx context.Context, list *model.CRMList) error {
	if err := r.db.WithContext(ctx).Create(list).Error; err != nil {
		return fmt.Errorf("create list: %w", err)
	}
	return nil
}

// Update updates a list.
func (r *CRMListRepository) Update(ctx context.Context, list *model.CRMList) error {
	if err := r.db.WithContext(ctx).Save(list).Error; err != nil {
		return fmt.Errorf("update list: %w", err)
	}
	return nil
}

// Delete removes a list and all its members.
func (r *CRMListRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("list_id = ?", id).Delete(&model.CRMListMember{}).Error; err != nil {
			return fmt.Errorf("delete list members: %w", err)
		}
		if err := tx.Where("id = ?", id).Delete(&model.CRMList{}).Error; err != nil {
			return fmt.Errorf("delete list: %w", err)
		}
		return nil
	})
}

// ListMembers returns members of a list with pagination.
func (r *CRMListRepository) ListMembers(ctx context.Context, listID string, pagination model.PMPagination) ([]model.CRMListMember, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMListMember{}).Where("list_id = ?", listID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count list members: %w", err)
	}

	var members []model.CRMListMember
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&members).Error; err != nil {
		return nil, 0, fmt.Errorf("list list members: %w", err)
	}
	return members, total, nil
}

// AddMember adds a member to a static list.
func (r *CRMListRepository) AddMember(ctx context.Context, member *model.CRMListMember) error {
	if err := r.db.WithContext(ctx).Create(member).Error; err != nil {
		return fmt.Errorf("add list member: %w", err)
	}
	return nil
}

// RemoveMember removes a member from a list.
func (r *CRMListRepository) RemoveMember(ctx context.Context, listID, objectID string) error {
	if err := r.db.WithContext(ctx).Where("list_id = ? AND object_id = ?", listID, objectID).Delete(&model.CRMListMember{}).Error; err != nil {
		return fmt.Errorf("remove list member: %w", err)
	}
	return nil
}

// CountMembers returns the count of members in a list.
func (r *CRMListRepository) CountMembers(ctx context.Context, listID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMListMember{}).Where("list_id = ?", listID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count list members: %w", err)
	}
	return count, nil
}

// UpdateMemberCount updates the member_count on a list.
func (r *CRMListRepository) UpdateMemberCount(ctx context.Context, listID string, count int) error {
	if err := r.db.WithContext(ctx).Model(&model.CRMList{}).Where("id = ?", listID).Update("member_count", count).Error; err != nil {
		return fmt.Errorf("update member count: %w", err)
	}
	return nil
}

// EvaluateSmartListCount counts objects matching filter criteria for smart lists.
// filter format: {"field": "value", "lifecycle_stage": "lead", ...}
func (r *CRMListRepository) EvaluateSmartListCount(ctx context.Context, workspaceID, objectType string, filterCriteria model.JSONB) (int64, error) {
	var table string
	switch objectType {
	case "contact":
		table = "crm_contacts"
	case "company":
		table = "crm_companies"
	case "deal":
		table = "crm_deals"
	default:
		return 0, fmt.Errorf("unsupported object type: %s", objectType)
	}

	query := r.db.WithContext(ctx).Table(table).Where("workspace_id = ?", workspaceID)

	// Apply simple equality filters from the criteria
	for key, value := range filterCriteria {
		if key == "" || value == nil {
			continue
		}
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("evaluate smart list: %w", err)
	}
	return count, nil
}
