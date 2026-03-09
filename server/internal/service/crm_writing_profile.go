package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMWritingProfileService contains CRM writing profile business logic.
type CRMWritingProfileService struct {
	profileRepo *repository.CRMWritingProfileRepository
}

// NewCRMWritingProfileService creates a new CRMWritingProfileService.
func NewCRMWritingProfileService(profileRepo *repository.CRMWritingProfileRepository) *CRMWritingProfileService {
	return &CRMWritingProfileService{profileRepo: profileRepo}
}

// List returns writing profiles for a workspace.
func (s *CRMWritingProfileService) List(ctx context.Context, workspaceID string) ([]model.CRMWritingProfile, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.profileRepo.List(ctx, workspaceID)
}

// GetByMemberID returns the writing profile for a member.
func (s *CRMWritingProfileService) GetByMemberID(ctx context.Context, workspaceID, memberID string) (*model.CRMWritingProfile, error) {
	if workspaceID == "" || memberID == "" {
		return nil, fmt.Errorf("workspace_id and member_id are required")
	}
	profile, err := s.profileRepo.GetByMemberID(ctx, workspaceID, memberID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, fmt.Errorf("writing profile not found")
	}
	return profile, nil
}

// Create creates or returns existing writing profile.
func (s *CRMWritingProfileService) Create(ctx context.Context, req model.CreateCRMWritingProfileRequest) (*model.CRMWritingProfile, error) {
	if req.WorkspaceID == "" || req.MemberID == "" {
		return nil, fmt.Errorf("workspace_id and member_id are required")
	}

	// Check if profile already exists.
	existing, err := s.profileRepo.GetByMemberID(ctx, req.WorkspaceID, req.MemberID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	profile := &model.CRMWritingProfile{
		WorkspaceID:     req.WorkspaceID,
		MemberID:        req.MemberID,
		StyleAttributes: model.JSONB(req.StyleAttributes),
		SampleCount:     0,
	}

	if err := s.profileRepo.Create(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// Update updates a writing profile.
func (s *CRMWritingProfileService) Update(ctx context.Context, id string, req model.UpdateCRMWritingProfileRequest) (*model.CRMWritingProfile, error) {
	profile, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, fmt.Errorf("writing profile not found")
	}

	if req.StyleAttributes != nil {
		profile.StyleAttributes = model.JSONB(req.StyleAttributes)
	}
	if req.SampleCount != nil {
		profile.SampleCount = *req.SampleCount
	}

	if err := s.profileRepo.Update(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// Delete removes a writing profile.
func (s *CRMWritingProfileService) Delete(ctx context.Context, id string) error {
	profile, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if profile == nil {
		return fmt.Errorf("writing profile not found")
	}
	return s.profileRepo.Delete(ctx, id)
}
