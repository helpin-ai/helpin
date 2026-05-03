package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type SupportInboxViewService struct {
	viewRepo    *repository.SupportInboxViewRepository
	wsPublisher *websocket.Publisher
	logger      *slog.Logger
}

func NewSupportInboxViewService(viewRepo *repository.SupportInboxViewRepository, wsPublisher *websocket.Publisher) *SupportInboxViewService {
	return &SupportInboxViewService{
		viewRepo:    viewRepo,
		wsPublisher: wsPublisher,
		logger:      slog.Default().With("service", "support_inbox_view"),
	}
}

func (s *SupportInboxViewService) List(ctx context.Context, workspaceID, userID string) ([]model.SupportInboxView, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.viewRepo.ListByWorkspace(ctx, workspaceID, userID)
}

func (s *SupportInboxViewService) Create(ctx context.Context, workspaceID, userID, role string, req model.CreateSupportInboxViewRequest) (*model.SupportInboxView, error) {
	name := strings.TrimSpace(req.Name)
	if strings.TrimSpace(workspaceID) == "" || name == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if req.IsShared && !canManageSharedSupportInboxViews(role) {
		return nil, fmt.Errorf("forbidden: only admins and owners can create shared views")
	}
	view := &model.SupportInboxView{
		WorkspaceID: workspaceID,
		Name:        name,
		Filters:     req.Filters,
		IsShared:    req.IsShared,
		CreatedBy:   userID,
	}
	if view.Filters == nil {
		view.Filters = model.SupportInboxViewFilters{}
	}
	if err := s.viewRepo.Create(ctx, view); err != nil {
		s.logger.ErrorContext(ctx, "failed to create support inbox view", "error", err, "workspace_id", workspaceID)
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "created", "support_inbox_view", view.ID, workspaceID, userID)
	return view, nil
}

func (s *SupportInboxViewService) Update(ctx context.Context, workspaceID, id, userID, role string, req model.UpdateSupportInboxViewRequest) (*model.SupportInboxView, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	view, err := s.viewRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, fmt.Errorf("view not found")
	}
	if !canModifySupportInboxView(view, userID, role) {
		return nil, fmt.Errorf("forbidden: you cannot update this view")
	}
	if req.IsShared != nil && *req.IsShared && !canManageSharedSupportInboxViews(role) {
		return nil, fmt.Errorf("forbidden: only admins and owners can share views")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		view.Name = name
	}
	if req.Filters != nil {
		view.Filters = *req.Filters
		if view.Filters == nil {
			view.Filters = model.SupportInboxViewFilters{}
		}
	}
	if req.IsShared != nil {
		view.IsShared = *req.IsShared
	}
	if err := s.viewRepo.Update(ctx, view); err != nil {
		s.logger.ErrorContext(ctx, "failed to update support inbox view", "error", err, "view_id", id)
		return nil, err
	}
	publishWorkspaceEvent(s.wsPublisher, "updated", "support_inbox_view", view.ID, view.WorkspaceID, userID)
	return view, nil
}

func (s *SupportInboxViewService) Delete(ctx context.Context, workspaceID, id, userID, role string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	view, err := s.viewRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("view not found")
	}
	if !canModifySupportInboxView(view, userID, role) {
		return fmt.Errorf("forbidden: you cannot delete this view")
	}
	if err := s.viewRepo.Delete(ctx, workspaceID, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete support inbox view", "error", err, "view_id", id)
		return err
	}
	publishWorkspaceEvent(s.wsPublisher, "deleted", "support_inbox_view", view.ID, view.WorkspaceID, userID)
	return nil
}

func canManageSharedSupportInboxViews(role string) bool {
	return role == model.RoleOwner || role == model.RoleAdmin
}

func canModifySupportInboxView(view *model.SupportInboxView, userID, role string) bool {
	if view == nil {
		return false
	}
	if !view.IsShared {
		return view.CreatedBy == userID
	}
	return view.CreatedBy == userID || canManageSharedSupportInboxViews(role)
}
