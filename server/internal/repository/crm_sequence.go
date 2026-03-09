package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMSequenceRepository handles DB operations for CRM sequences.
type CRMSequenceRepository struct {
	db *gorm.DB
}

// NewCRMSequenceRepository creates a new CRMSequenceRepository.
func NewCRMSequenceRepository(db *gorm.DB) *CRMSequenceRepository {
	return &CRMSequenceRepository{db: db}
}

// ── Sequences ──

// Create inserts a sequence.
func (r *CRMSequenceRepository) Create(ctx context.Context, sequence *model.CRMSequence) error {
	if err := r.db.WithContext(ctx).Create(sequence).Error; err != nil {
		return fmt.Errorf("create sequence: %w", err)
	}
	return nil
}

// GetByID returns a sequence by ID.
func (r *CRMSequenceRepository) GetByID(ctx context.Context, id string) (*model.CRMSequence, error) {
	var sequence model.CRMSequence
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&sequence).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get sequence: %w", err)
	}
	return &sequence, nil
}

// List returns sequences with optional filters.
func (r *CRMSequenceRepository) List(ctx context.Context, workspaceID string, filters model.CRMSequenceListFilters, pagination model.PMPagination) ([]model.CRMSequence, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMSequence{}).Where("workspace_id = ?", workspaceID)

	if filters.Status != nil && *filters.Status != "" {
		query = query.Where("status = ?", *filters.Status)
	}
	if filters.Search != nil && *filters.Search != "" {
		search := "%" + *filters.Search + "%"
		query = query.Where("name ILIKE ?", search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count sequences: %w", err)
	}

	var sequences []model.CRMSequence
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&sequences).Error; err != nil {
		return nil, 0, fmt.Errorf("list sequences: %w", err)
	}
	return sequences, total, nil
}

// Update updates a sequence.
func (r *CRMSequenceRepository) Update(ctx context.Context, sequence *model.CRMSequence) error {
	if err := r.db.WithContext(ctx).Save(sequence).Error; err != nil {
		return fmt.Errorf("update sequence: %w", err)
	}
	return nil
}

// Delete removes a sequence.
func (r *CRMSequenceRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMSequence{}).Error; err != nil {
		return fmt.Errorf("delete sequence: %w", err)
	}
	return nil
}

// ── Enrollments ──

// CreateEnrollment inserts an enrollment.
func (r *CRMSequenceRepository) CreateEnrollment(ctx context.Context, enrollment *model.CRMSequenceEnrollment) error {
	if err := r.db.WithContext(ctx).Create(enrollment).Error; err != nil {
		return fmt.Errorf("create enrollment: %w", err)
	}
	return nil
}

// GetEnrollmentByID returns an enrollment by ID.
func (r *CRMSequenceRepository) GetEnrollmentByID(ctx context.Context, id string) (*model.CRMSequenceEnrollment, error) {
	var enrollment model.CRMSequenceEnrollment
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get enrollment: %w", err)
	}
	return &enrollment, nil
}

// ListEnrollments returns enrollments with optional filters.
func (r *CRMSequenceRepository) ListEnrollments(ctx context.Context, workspaceID string, filters model.CRMSequenceEnrollmentListFilters, pagination model.PMPagination) ([]model.CRMSequenceEnrollment, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMSequenceEnrollment{}).Where("workspace_id = ?", workspaceID)

	if filters.SequenceID != nil && *filters.SequenceID != "" {
		query = query.Where("sequence_id = ?", *filters.SequenceID)
	}
	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where("contact_id = ?", *filters.ContactID)
	}
	if filters.Status != nil && *filters.Status != "" {
		query = query.Where("status = ?", *filters.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count enrollments: %w", err)
	}

	var enrollments []model.CRMSequenceEnrollment
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("enrolled_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&enrollments).Error; err != nil {
		return nil, 0, fmt.Errorf("list enrollments: %w", err)
	}
	return enrollments, total, nil
}

// UpdateEnrollment updates an enrollment.
func (r *CRMSequenceRepository) UpdateEnrollment(ctx context.Context, enrollment *model.CRMSequenceEnrollment) error {
	if err := r.db.WithContext(ctx).Save(enrollment).Error; err != nil {
		return fmt.Errorf("update enrollment: %w", err)
	}
	return nil
}

// DeleteEnrollment removes an enrollment.
func (r *CRMSequenceRepository) DeleteEnrollment(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMSequenceEnrollment{}).Error; err != nil {
		return fmt.Errorf("delete enrollment: %w", err)
	}
	return nil
}
