package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
)

// CRMContactRepository handles DB operations for CRM contacts.
type CRMContactRepository struct {
	db *gorm.DB
}

// NewCRMContactRepository creates a new CRMContactRepository.
func NewCRMContactRepository(db *gorm.DB) *CRMContactRepository {
	return &CRMContactRepository{db: db}
}

// CountByWorkspace returns the number of contacts in a workspace.
func (r *CRMContactRepository) CountByWorkspace(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMContact{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count contacts: %w", err)
	}
	return count, nil
}

// GetNextDisplayID generates the next sequential display ID for contacts in a workspace.
func (r *CRMContactRepository) GetNextDisplayID(ctx context.Context, workspaceID string) (string, error) {
	count, err := r.CountByWorkspace(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CON-%d", count+1), nil
}

// List returns contacts in a workspace with optional filters.
func (r *CRMContactRepository) List(ctx context.Context, workspaceID string, filters model.CRMContactListFilters, pagination model.PMPagination) ([]model.CRMContact, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMContact{}).Where("workspace_id = ?", workspaceID)

	if filters.LifecycleStage != nil && *filters.LifecycleStage != "" {
		query = query.Where("lifecycle_stage = ?", *filters.LifecycleStage)
	}
	if filters.LeadStatus != nil && *filters.LeadStatus != "" {
		query = query.Where("lead_status = ?", *filters.LeadStatus)
	}
	if filters.OwnerMemberID != nil && *filters.OwnerMemberID != "" {
		query = query.Where("owner_member_id = ?", *filters.OwnerMemberID)
	}
	if filters.Search != nil && *filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(*filters.Search)) + "%"
		query = query.Where(
			"(LOWER(first_name) LIKE ? OR LOWER(COALESCE(last_name, '')) LIKE ? OR LOWER(COALESCE(email, '')) LIKE ? OR LOWER(COALESCE(job_title, '')) LIKE ?)",
			search,
			search,
			search,
			search,
		)
	}
	if filters.Query != nil {
		var err error
		query, err = querybuilder.ApplyGORM(query, filters.Query, crmContactFilterDefinitions)
		if err != nil {
			return nil, 0, err
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contacts: %w", err)
	}

	var contacts []model.CRMContact
	offset := (pagination.Page - 1) * pagination.PerPage
	if pagination.Offset != nil {
		offset = *pagination.Offset
	}
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&contacts).Error; err != nil {
		return nil, 0, fmt.Errorf("list contacts: %w", err)
	}
	return contacts, total, nil
}

// GetByID returns a contact by ID.
func (r *CRMContactRepository) GetByID(ctx context.Context, id string) (*model.CRMContact, error) {
	var contact model.CRMContact
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&contact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get contact: %w", err)
	}
	return &contact, nil
}

// ListByIDs returns contacts by ID for a workspace.
func (r *CRMContactRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.CRMContact, error) {
	if len(ids) == 0 {
		return []model.CRMContact{}, nil
	}
	var contacts []model.CRMContact
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("list contacts by ids: %w", err)
	}
	return contacts, nil
}

// Create inserts a contact.
func (r *CRMContactRepository) Create(ctx context.Context, contact *model.CRMContact) error {
	if err := r.db.WithContext(ctx).Create(contact).Error; err != nil {
		return fmt.Errorf("create contact: %w", err)
	}
	return nil
}

// CreateInBatches inserts a set of contacts in batches.
func (r *CRMContactRepository) CreateInBatches(ctx context.Context, contacts []model.CRMContact, batchSize int) error {
	if len(contacts) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(contacts, batchSize).Error; err != nil {
		return fmt.Errorf("create contacts in batches: %w", err)
	}
	return nil
}

// GetByEmail returns the first contact in a workspace whose email matches exactly
// (case-insensitive).
func (r *CRMContactRepository) GetByEmail(ctx context.Context, workspaceID, email string) (*model.CRMContact, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, nil
	}

	var contact model.CRMContact
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND email IS NOT NULL AND LOWER(email) = ?", workspaceID, email).
		Order("created_at ASC, id ASC").
		First(&contact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get contact by email: %w", err)
	}
	return &contact, nil
}

// ListByEmails returns contacts in a workspace keyed by normalized email.
func (r *CRMContactRepository) ListByEmails(ctx context.Context, workspaceID string, emails []string) (map[string]model.CRMContact, error) {
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		email = strings.TrimSpace(strings.ToLower(email))
		if email == "" {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		normalized = append(normalized, email)
	}
	if len(normalized) == 0 {
		return map[string]model.CRMContact{}, nil
	}

	var contacts []model.CRMContact
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND email IS NOT NULL AND LOWER(email) IN ?", workspaceID, normalized).
		Order("created_at ASC, id ASC").
		Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("list contacts by email: %w", err)
	}

	result := make(map[string]model.CRMContact, len(contacts))
	for _, contact := range contacts {
		if contact.Email == nil {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(*contact.Email))
		if key == "" {
			continue
		}
		if _, exists := result[key]; exists {
			continue
		}
		result[key] = contact
	}
	return result, nil
}

// Update updates a contact.
func (r *CRMContactRepository) Update(ctx context.Context, contact *model.CRMContact) error {
	if err := r.db.WithContext(ctx).Save(contact).Error; err != nil {
		return fmt.Errorf("update contact: %w", err)
	}
	return nil
}

// Delete removes a contact.
func (r *CRMContactRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMContact{}).Error; err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	return nil
}

// MarkEmailInvalid flags every contact in a workspace whose email matches
// (case-insensitive) as having an undeliverable email. No-op when the email
// is empty or no matching contact exists. Idempotent — already-invalid rows
// are updated only if the reason changed.
func (r *CRMContactRepository) MarkEmailInvalid(ctx context.Context, workspaceID, email, reason string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if workspaceID == "" || email == "" {
		return nil
	}
	reason = strings.TrimSpace(reason)
	now := time.Now().UTC()
	updates := map[string]any{
		"email_status":            model.CRMContactEmailStatusInvalid,
		"email_status_reason":     reason,
		"email_status_updated_at": now,
	}
	if err := r.db.WithContext(ctx).
		Model(&model.CRMContact{}).
		Where(
			"workspace_id = ? AND email IS NOT NULL AND LOWER(email) = ? AND (email_status <> ? OR COALESCE(email_status_reason, '') <> ?)",
			workspaceID, email, model.CRMContactEmailStatusInvalid, reason,
		).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("mark contact email invalid: %w", err)
	}
	return nil
}
