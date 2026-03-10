package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

// WorkspaceService handles workspace business logic.
type WorkspaceService struct {
	workspaceRepo       *repository.WorkspaceRepository
	attachmentRepo      *repository.PMAttachmentRepository
	s3Client            *storage.S3Client
	defaultsInitializer WorkspaceDefaultsInitializer
	logger              *slog.Logger
}

// WorkspaceDefaultsInitializer seeds default workspace-scoped data after creation.
type WorkspaceDefaultsInitializer interface {
	SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error
}

// NewWorkspaceService creates a new WorkspaceService.
func NewWorkspaceService(workspaceRepo *repository.WorkspaceRepository, attachmentRepo *repository.PMAttachmentRepository, s3Client *storage.S3Client, defaultsInitializer ...WorkspaceDefaultsInitializer) *WorkspaceService {
	var initializer WorkspaceDefaultsInitializer
	if len(defaultsInitializer) > 0 {
		initializer = defaultsInitializer[0]
	}
	return &WorkspaceService{
		workspaceRepo:       workspaceRepo,
		attachmentRepo:      attachmentRepo,
		s3Client:            s3Client,
		defaultsInitializer: initializer,
		logger:              slog.Default().With("service", "workspace"),
	}
}

// Create creates a workspace and adds the creator as the owner member.
func (s *WorkspaceService) Create(ctx context.Context, req model.CreateWorkspaceRequest, ownerID string) (*model.WorkspaceWithRole, error) {
	if req.Name == "" || req.Slug == "" {
		return nil, fmt.Errorf("name and slug are required")
	}

	var orgID *string
	if req.OrganizationID != "" {
		orgID = &req.OrganizationID
	}

	ws, err := s.workspaceRepo.Create(ctx, req.Name, req.Slug, ownerID, orgID, req.Description, req.Timezone)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create workspace", "error", err, "slug", req.Slug)
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	_, err = s.workspaceRepo.AddMember(ctx, ws.ID, ownerID, "owner")
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to add owner as member", "error", err, "workspace_id", ws.ID, "user_id", ownerID)
		return nil, fmt.Errorf("add owner as member: %w", err)
	}

	if s.defaultsInitializer != nil {
		if err := s.defaultsInitializer.SeedWorkspaceDefaults(ctx, ws.ID, ownerID); err != nil {
			s.logger.ErrorContext(ctx, "failed to seed workspace defaults", "error", err, "workspace_id", ws.ID)
			return nil, fmt.Errorf("seed workspace defaults: %w", err)
		}
	}

	s.logger.InfoContext(ctx, "workspace created", "workspace_id", ws.ID, "name", ws.Name, "slug", ws.Slug)

	return &model.WorkspaceWithRole{
		Workspace: *ws,
		Role:      "owner",
	}, nil
}

// List returns all workspaces the user belongs to. If organizationID is non-empty, filters by org.
func (s *WorkspaceService) List(ctx context.Context, userID string, organizationID string) ([]model.WorkspaceWithRole, error) {
	return s.workspaceRepo.List(ctx, userID, organizationID)
}

// GetBySlug returns a workspace by its slug.
func (s *WorkspaceService) GetBySlug(ctx context.Context, slug string) (*model.Workspace, error) {
	ws, err := s.workspaceRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

// GetByID returns a workspace by its ID.
func (s *WorkspaceService) GetByID(ctx context.Context, id string) (*model.Workspace, error) {
	ws, err := s.workspaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

// Update modifies a workspace.
func (s *WorkspaceService) Update(ctx context.Context, id string, req model.UpdateWorkspaceRequest) (*model.Workspace, error) {
	ws, err := s.workspaceRepo.Update(ctx, id, req.Name, req.Description, req.LogoURL, req.Timezone)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to update workspace", "error", err, "workspace_id", id)
		return nil, err
	}
	s.logger.InfoContext(ctx, "workspace updated", "workspace_id", id)
	return ws, nil
}

// UploadLogo uploads a workspace logo to S3 and saves the public URL.
func (s *WorkspaceService) UploadLogo(ctx context.Context, id string, body io.Reader, size int64, contentType string) (*model.Workspace, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("file storage not configured")
	}

	ext := ".png"
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/svg+xml":
		ext = ".svg"
	}
	key := fmt.Sprintf("workspaces/%s/logo/%s%s", id, uuid.New().String(), ext)

	if err := s.s3Client.PutObject(ctx, key, contentType, size, body, true); err != nil {
		return nil, fmt.Errorf("upload logo: %w", err)
	}

	logoURL := s.s3Client.PublicURL(key)
	return s.workspaceRepo.Update(ctx, id, nil, nil, &logoURL, nil)
}

// DeleteLogo removes the workspace logo.
func (s *WorkspaceService) DeleteLogo(ctx context.Context, id string) (*model.Workspace, error) {
	ws, err := s.workspaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, fmt.Errorf("workspace not found")
	}

	// Delete old logo from S3 if it exists
	if ws.LogoURL != nil && *ws.LogoURL != "" && s.s3Client != nil {
		key := filepath.Base(*ws.LogoURL)
		// Extract the full key from the URL path
		_ = s.s3Client.DeleteObject(ctx, fmt.Sprintf("workspaces/%s/logo/%s", id, key))
	}

	empty := ""
	return s.workspaceRepo.Update(ctx, id, nil, nil, &empty, nil)
}

// Delete removes a workspace and all associated data including S3 attachments.
func (s *WorkspaceService) Delete(ctx context.Context, id string) error {
	// Clean up S3 attachments before cascade-deleting DB records.
	if s.attachmentRepo != nil && s.s3Client != nil {
		attachments, _ := s.attachmentRepo.ListByWorkspace(ctx, id)
		for _, a := range attachments {
			if a.StorageKey != "" {
				_ = s.s3Client.DeleteObject(ctx, a.StorageKey)
			}
		}
	}
	if err := s.workspaceRepo.Delete(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete workspace", "error", err, "workspace_id", id)
		return err
	}
	s.logger.InfoContext(ctx, "workspace deleted", "workspace_id", id)
	return nil
}

// GetMyRole returns the user's role in a workspace.
func (s *WorkspaceService) GetMyRole(ctx context.Context, workspaceID, userID string) (string, error) {
	return s.workspaceRepo.GetMemberRole(ctx, workspaceID, userID)
}

// GetMyMembership returns the user's full membership in a workspace.
func (s *WorkspaceService) GetMyMembership(ctx context.Context, workspaceID, userID string) (*model.WorkspaceMember, error) {
	m, err := s.workspaceRepo.GetMembership(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("membership not found")
	}
	return m, nil
}

// ListMembers returns all members of a workspace with user details.
func (s *WorkspaceService) ListMembers(ctx context.Context, workspaceID string) ([]model.MemberWithUser, error) {
	return s.workspaceRepo.ListMembers(ctx, workspaceID)
}

// ListAssignableMembers returns joined and pending workspace identities for PM pickers.
func (s *WorkspaceService) ListAssignableMembers(ctx context.Context, workspaceID string) ([]model.AssignableMember, error) {
	return s.workspaceRepo.ListAssignableMembers(ctx, workspaceID)
}
