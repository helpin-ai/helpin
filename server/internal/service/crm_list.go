package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMListService contains CRM list business logic.
type CRMListService struct {
	listRepo *repository.CRMListRepository
}

// NewCRMListService creates a new CRMListService.
func NewCRMListService(listRepo *repository.CRMListRepository) *CRMListService {
	return &CRMListService{listRepo: listRepo}
}

// List returns lists with filters and pagination.
func (s *CRMListService) List(ctx context.Context, workspaceID string, filters model.CRMListFilters, pagination model.PMPagination) ([]model.CRMList, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.listRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a list by ID.
func (s *CRMListService) GetByID(ctx context.Context, id string) (*model.CRMList, error) {
	list, err := s.listRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, fmt.Errorf("list not found")
	}
	return list, nil
}

// Create creates a list.
func (s *CRMListService) Create(ctx context.Context, req model.CreateCRMListRequest) (*model.CRMList, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if req.ListType != model.CRMListTypeStatic && req.ListType != model.CRMListTypeSmart {
		return nil, fmt.Errorf("list_type must be 'static' or 'smart'")
	}
	if req.ObjectType != "contact" && req.ObjectType != "company" && req.ObjectType != "deal" {
		return nil, fmt.Errorf("object_type must be 'contact', 'company', or 'deal'")
	}

	list := &model.CRMList{
		WorkspaceID:    req.WorkspaceID,
		Name:           strings.TrimSpace(req.Name),
		ListType:       req.ListType,
		ObjectType:     req.ObjectType,
		FilterCriteria: model.JSONB(req.FilterCriteria),
	}

	if err := s.listRepo.Create(ctx, list); err != nil {
		return nil, err
	}

	// For smart lists, evaluate count immediately
	if list.ListType == model.CRMListTypeSmart {
		count, err := s.listRepo.EvaluateSmartListCount(ctx, list.WorkspaceID, list.ObjectType, list.FilterCriteria)
		if err == nil {
			list.MemberCount = int(count)
			_ = s.listRepo.UpdateMemberCount(ctx, list.ID, int(count))
		}
	}

	return list, nil
}

// Update updates a list.
func (s *CRMListService) Update(ctx context.Context, id string, req model.UpdateCRMListRequest) (*model.CRMList, error) {
	list, err := s.listRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, fmt.Errorf("list not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		list.Name = name
	}
	if req.FilterCriteria != nil {
		list.FilterCriteria = model.JSONB(req.FilterCriteria)
	}

	if err := s.listRepo.Update(ctx, list); err != nil {
		return nil, err
	}

	// For smart lists, recalculate member count after filter update
	if list.ListType == model.CRMListTypeSmart && req.FilterCriteria != nil {
		count, err := s.listRepo.EvaluateSmartListCount(ctx, list.WorkspaceID, list.ObjectType, list.FilterCriteria)
		if err == nil {
			list.MemberCount = int(count)
			_ = s.listRepo.UpdateMemberCount(ctx, list.ID, int(count))
		}
	}

	return list, nil
}

// Delete removes a list.
func (s *CRMListService) Delete(ctx context.Context, id string) error {
	list, err := s.listRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if list == nil {
		return fmt.Errorf("list not found")
	}
	return s.listRepo.Delete(ctx, id)
}

// ListMembers returns members of a list with pagination.
func (s *CRMListService) ListMembers(ctx context.Context, listID string, pagination model.PMPagination) ([]model.CRMListMember, int64, error) {
	list, err := s.listRepo.GetByID(ctx, listID)
	if err != nil {
		return nil, 0, err
	}
	if list == nil {
		return nil, 0, fmt.Errorf("list not found")
	}
	return s.listRepo.ListMembers(ctx, listID, pagination)
}

// AddMember adds a member to a static list.
func (s *CRMListService) AddMember(ctx context.Context, listID, objectID string) (*model.CRMListMember, error) {
	list, err := s.listRepo.GetByID(ctx, listID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, fmt.Errorf("list not found")
	}
	if list.ListType != model.CRMListTypeStatic {
		return nil, fmt.Errorf("cannot manually add members to a smart list")
	}

	member := &model.CRMListMember{
		ListID:   listID,
		ObjectID: objectID,
	}

	if err := s.listRepo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	// Update member count
	count, err := s.listRepo.CountMembers(ctx, listID)
	if err == nil {
		_ = s.listRepo.UpdateMemberCount(ctx, listID, int(count))
	}

	return member, nil
}

// RemoveMember removes a member from a list.
func (s *CRMListService) RemoveMember(ctx context.Context, listID, objectID string) error {
	list, err := s.listRepo.GetByID(ctx, listID)
	if err != nil {
		return err
	}
	if list == nil {
		return fmt.Errorf("list not found")
	}
	if list.ListType != model.CRMListTypeStatic {
		return fmt.Errorf("cannot manually remove members from a smart list")
	}

	if err := s.listRepo.RemoveMember(ctx, listID, objectID); err != nil {
		return err
	}

	// Update member count
	count, err := s.listRepo.CountMembers(ctx, listID)
	if err == nil {
		_ = s.listRepo.UpdateMemberCount(ctx, listID, int(count))
	}

	return nil
}
