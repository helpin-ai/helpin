package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMSequenceService contains CRM sequence business logic.
type CRMSequenceService struct {
	sequenceRepo *repository.CRMSequenceRepository
}

// NewCRMSequenceService creates a new CRMSequenceService.
func NewCRMSequenceService(sequenceRepo *repository.CRMSequenceRepository) *CRMSequenceService {
	return &CRMSequenceService{sequenceRepo: sequenceRepo}
}

// ── Sequences ──

// List returns sequences with filters and pagination.
func (s *CRMSequenceService) List(ctx context.Context, workspaceID string, filters model.CRMSequenceListFilters, pagination model.PMPagination) ([]model.CRMSequence, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.sequenceRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a sequence by ID.
func (s *CRMSequenceService) GetByID(ctx context.Context, id string) (*model.CRMSequence, error) {
	seq, err := s.sequenceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if seq == nil {
		return nil, fmt.Errorf("sequence not found")
	}
	return seq, nil
}

// Create creates a new sequence.
func (s *CRMSequenceService) Create(ctx context.Context, req model.CreateCRMSequenceRequest) (*model.CRMSequence, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}

	status := model.CRMSequenceStatusDraft
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}

	seq := &model.CRMSequence{
		WorkspaceID: req.WorkspaceID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Status:      status,
		Steps:       model.JSONB(req.Steps),
		CreatedBy:   req.CreatedBy,
	}

	if err := s.sequenceRepo.Create(ctx, seq); err != nil {
		return nil, err
	}
	return seq, nil
}

// Update updates a sequence.
func (s *CRMSequenceService) Update(ctx context.Context, id string, req model.UpdateCRMSequenceRequest) (*model.CRMSequence, error) {
	seq, err := s.sequenceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if seq == nil {
		return nil, fmt.Errorf("sequence not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		seq.Name = name
	}
	if req.Description != nil {
		seq.Description = req.Description
	}
	if req.Status != nil {
		seq.Status = *req.Status
	}
	if req.Steps != nil {
		seq.Steps = model.JSONB(req.Steps)
	}

	if err := s.sequenceRepo.Update(ctx, seq); err != nil {
		return nil, err
	}
	return seq, nil
}

// Delete removes a sequence.
func (s *CRMSequenceService) Delete(ctx context.Context, id string) error {
	seq, err := s.sequenceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if seq == nil {
		return fmt.Errorf("sequence not found")
	}
	return s.sequenceRepo.Delete(ctx, id)
}

// ── Enrollments ──

// ListEnrollments returns enrollments with filters and pagination.
func (s *CRMSequenceService) ListEnrollments(ctx context.Context, workspaceID string, filters model.CRMSequenceEnrollmentListFilters, pagination model.PMPagination) ([]model.CRMSequenceEnrollment, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.sequenceRepo.ListEnrollments(ctx, workspaceID, filters, pagination)
}

// CreateEnrollment enrolls a contact in a sequence.
func (s *CRMSequenceService) CreateEnrollment(ctx context.Context, req model.CreateCRMSequenceEnrollmentRequest) (*model.CRMSequenceEnrollment, error) {
	if req.WorkspaceID == "" || req.SequenceID == "" || req.ContactID == "" {
		return nil, fmt.Errorf("workspace_id, sequence_id, and contact_id are required")
	}

	enrollment := &model.CRMSequenceEnrollment{
		WorkspaceID: req.WorkspaceID,
		SequenceID:  req.SequenceID,
		ContactID:   req.ContactID,
		CurrentStep: 0,
		Status:      model.CRMEnrollmentStatusActive,
		EnrolledAt:  time.Now(),
	}

	if err := s.sequenceRepo.CreateEnrollment(ctx, enrollment); err != nil {
		return nil, err
	}

	// Increment enrollment count on the sequence.
	seq, err := s.sequenceRepo.GetByID(ctx, req.SequenceID)
	if err == nil && seq != nil {
		seq.EnrollmentCount++
		_ = s.sequenceRepo.Update(ctx, seq)
	}

	return enrollment, nil
}

// UpdateEnrollment updates an enrollment status.
func (s *CRMSequenceService) UpdateEnrollment(ctx context.Context, id string, req model.UpdateCRMSequenceEnrollmentRequest) (*model.CRMSequenceEnrollment, error) {
	enrollment, err := s.sequenceRepo.GetEnrollmentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if enrollment == nil {
		return nil, fmt.Errorf("enrollment not found")
	}

	if req.Status != nil {
		enrollment.Status = *req.Status
		if *req.Status == model.CRMEnrollmentStatusCompleted {
			now := time.Now()
			enrollment.CompletedAt = &now
		}
	}
	if req.ExitReason != nil {
		enrollment.ExitReason = req.ExitReason
	}

	if err := s.sequenceRepo.UpdateEnrollment(ctx, enrollment); err != nil {
		return nil, err
	}
	return enrollment, nil
}

// DeleteEnrollment removes an enrollment.
func (s *CRMSequenceService) DeleteEnrollment(ctx context.Context, id string) error {
	enrollment, err := s.sequenceRepo.GetEnrollmentByID(ctx, id)
	if err != nil {
		return err
	}
	if enrollment == nil {
		return fmt.Errorf("enrollment not found")
	}
	return s.sequenceRepo.DeleteEnrollment(ctx, id)
}
