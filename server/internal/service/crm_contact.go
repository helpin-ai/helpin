package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMContactService contains CRM contact business logic.
type CRMContactService struct {
	contactRepo *repository.CRMContactRepository
}

// NewCRMContactService creates a new CRMContactService.
func NewCRMContactService(contactRepo *repository.CRMContactRepository) *CRMContactService {
	return &CRMContactService{contactRepo: contactRepo}
}

// List returns contacts with filters and pagination.
func (s *CRMContactService) List(ctx context.Context, workspaceID string, filters model.CRMContactListFilters, pagination model.PMPagination) ([]model.CRMContact, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.contactRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a contact by ID.
func (s *CRMContactService) GetByID(ctx context.Context, id string) (*model.CRMContact, error) {
	contact, err := s.contactRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if contact == nil {
		return nil, fmt.Errorf("contact not found")
	}
	return contact, nil
}

// Create creates a contact.
func (s *CRMContactService) Create(ctx context.Context, req model.CreateCRMContactRequest) (*model.CRMContact, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.FirstName) == "" {
		return nil, fmt.Errorf("workspace_id and first_name are required")
	}

	displayID, err := s.contactRepo.GetNextDisplayID(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}

	lifecycleStage := model.CRMLifecycleSubscriber
	if req.LifecycleStage != nil && *req.LifecycleStage != "" {
		lifecycleStage = *req.LifecycleStage
	}
	leadStatus := model.CRMLeadStatusNew
	if req.LeadStatus != nil && *req.LeadStatus != "" {
		leadStatus = *req.LeadStatus
	}

	contact := &model.CRMContact{
		WorkspaceID:      req.WorkspaceID,
		DisplayID:        displayID,
		FirstName:        strings.TrimSpace(req.FirstName),
		LastName:         req.LastName,
		Email:            req.Email,
		Phone:            req.Phone,
		JobTitle:         req.JobTitle,
		LifecycleStage:   lifecycleStage,
		LeadStatus:       leadStatus,
		OwnerMemberID:    req.OwnerMemberID,
		AvatarURL:        req.AvatarURL,
		Source:           req.Source,
		CustomProperties: model.JSONB(req.CustomProperties),
	}

	if err := s.contactRepo.Create(ctx, contact); err != nil {
		return nil, err
	}
	return contact, nil
}

// Update updates a contact.
func (s *CRMContactService) Update(ctx context.Context, id string, req model.UpdateCRMContactRequest) (*model.CRMContact, error) {
	contact, err := s.contactRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if contact == nil {
		return nil, fmt.Errorf("contact not found")
	}

	if req.FirstName != nil {
		name := strings.TrimSpace(*req.FirstName)
		if name == "" {
			return nil, fmt.Errorf("first_name cannot be empty")
		}
		contact.FirstName = name
	}
	if req.LastName != nil {
		contact.LastName = req.LastName
	}
	if req.Email != nil {
		contact.Email = req.Email
	}
	if req.Phone != nil {
		contact.Phone = req.Phone
	}
	if req.JobTitle != nil {
		contact.JobTitle = req.JobTitle
	}
	if req.LifecycleStage != nil {
		contact.LifecycleStage = *req.LifecycleStage
	}
	if req.LeadStatus != nil {
		contact.LeadStatus = *req.LeadStatus
	}
	if req.OwnerMemberID != nil {
		contact.OwnerMemberID = req.OwnerMemberID
	}
	if req.AvatarURL != nil {
		contact.AvatarURL = req.AvatarURL
	}
	if req.Source != nil {
		contact.Source = req.Source
	}
	if req.CustomProperties != nil {
		contact.CustomProperties = model.JSONB(req.CustomProperties)
	}

	if err := s.contactRepo.Update(ctx, contact); err != nil {
		return nil, err
	}
	return contact, nil
}

// Delete removes a contact.
func (s *CRMContactService) Delete(ctx context.Context, id string) error {
	contact, err := s.contactRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if contact == nil {
		return fmt.Errorf("contact not found")
	}
	return s.contactRepo.Delete(ctx, id)
}
