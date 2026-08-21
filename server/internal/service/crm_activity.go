package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMActivityService contains CRM activity business logic.
type CRMActivityService struct {
	activityRepo *repository.CRMActivityRepository
}

// NewCRMActivityService creates a new CRMActivityService.
func NewCRMActivityService(activityRepo *repository.CRMActivityRepository) *CRMActivityService {
	return &CRMActivityService{activityRepo: activityRepo}
}

// List returns activities with filters and pagination.
func (s *CRMActivityService) List(ctx context.Context, workspaceID string, filters model.CRMActivityListFilters, pagination model.PMPagination) ([]model.CRMActivity, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.activityRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns an activity by ID.
func (s *CRMActivityService) GetByID(ctx context.Context, id string) (*model.CRMActivity, error) {
	activity, err := s.activityRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, fmt.Errorf("activity not found")
	}
	return activity, nil
}

// Create creates an activity.
func (s *CRMActivityService) Create(ctx context.Context, req model.CreateCRMActivityRequest) (*model.CRMActivity, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if !isValidActivityType(req.ActivityType) {
		return nil, fmt.Errorf("invalid activity_type")
	}

	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}

	activity := &model.CRMActivity{
		WorkspaceID:   req.WorkspaceID,
		ActivityType:  req.ActivityType,
		ContactID:     req.ContactID,
		CompanyID:     req.CompanyID,
		DealID:        req.DealID,
		OwnerMemberID: req.OwnerMemberID,
		Subject:       req.Subject,
		Body:          req.Body,
		OccurredAt:    occurredAt,
		Metadata:      model.JSONB(req.Metadata),
	}

	if err := s.activityRepo.Create(ctx, activity); err != nil {
		return nil, err
	}
	return activity, nil
}

// Update updates an activity.
func (s *CRMActivityService) Update(ctx context.Context, id string, req model.UpdateCRMActivityRequest) (*model.CRMActivity, error) {
	activity, err := s.activityRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, fmt.Errorf("activity not found")
	}
	if activity.Metadata != nil && activity.Metadata["immutable"] == true {
		return nil, fmt.Errorf("system activity is immutable")
	}

	if req.ActivityType != nil {
		if !isValidActivityType(*req.ActivityType) {
			return nil, fmt.Errorf("invalid activity_type")
		}
		activity.ActivityType = *req.ActivityType
	}
	if req.ContactID != nil {
		activity.ContactID = req.ContactID
	}
	if req.CompanyID != nil {
		activity.CompanyID = req.CompanyID
	}
	if req.DealID != nil {
		activity.DealID = req.DealID
	}
	if req.Subject != nil {
		activity.Subject = req.Subject
	}
	if req.Body != nil {
		activity.Body = req.Body
	}
	if req.OccurredAt != nil {
		activity.OccurredAt = *req.OccurredAt
	}
	if req.Metadata != nil {
		activity.Metadata = model.JSONB(req.Metadata)
	}

	if err := s.activityRepo.Update(ctx, activity); err != nil {
		return nil, err
	}
	return activity, nil
}

// Delete removes an activity.
func (s *CRMActivityService) Delete(ctx context.Context, id string) error {
	activity, err := s.activityRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if activity == nil {
		return fmt.Errorf("activity not found")
	}
	return s.activityRepo.Delete(ctx, id)
}

func isValidActivityType(t string) bool {
	switch t {
	case model.CRMActivityNote, model.CRMActivityCall, model.CRMActivityMeeting, model.CRMActivityEmail:
		return true
	default:
		return false
	}
}
