package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var workspaceKeyPattern = regexp.MustCompile(`^[A-Z]{2,5}$`)

// WorkspaceService handles workspace business logic.
type WorkspaceService struct {
	workspaceRepo       *repository.WorkspaceRepository
	attachmentRepo      *repository.PMAttachmentRepository
	s3Client            *storage.S3Client
	defaultsInitializer WorkspaceDefaultsInitializer
	presence            websocket.PresenceProvider
	statusOverrideRepo  *repository.SupportTeammateStatusOverrideRepository
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

func (s *WorkspaceService) SetPresenceProvider(p websocket.PresenceProvider) {
	s.presence = p
}

func (s *WorkspaceService) SetStatusOverrideRepo(repo *repository.SupportTeammateStatusOverrideRepository) {
	s.statusOverrideRepo = repo
}

// Create creates a workspace and adds the creator as the owner member.
func (s *WorkspaceService) Create(ctx context.Context, req model.CreateWorkspaceRequest, ownerID string) (*model.WorkspaceWithRole, error) {
	if req.Name == "" || req.Slug == "" {
		return nil, fmt.Errorf("name and slug are required")
	}

	// Validate and normalize workspace key. Auto-generate from name if empty.
	req.WorkspaceKey = strings.ToUpper(strings.TrimSpace(req.WorkspaceKey))
	if req.WorkspaceKey == "" {
		alpha := strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				return r
			}
			return -1
		}, req.Name)
		if len(alpha) >= 2 {
			req.WorkspaceKey = strings.ToUpper(alpha[:min(3, len(alpha))])
		} else {
			req.WorkspaceKey = "WS"
		}
	}
	if !workspaceKeyPattern.MatchString(req.WorkspaceKey) {
		return nil, fmt.Errorf("workspace_key must be 2-5 uppercase letters")
	}
	// If key is taken, try appending letters A-Z to find an available one.
	baseKey := req.WorkspaceKey
	var orgIDPtr *string
	if req.OrganizationID != "" {
		orgIDPtr = &req.OrganizationID
	}
	available, err := s.workspaceRepo.IsWorkspaceKeyAvailable(ctx, req.WorkspaceKey, "", orgIDPtr)
	if err != nil {
		return nil, fmt.Errorf("check workspace key: %w", err)
	}
	if !available {
		found := false
		for c := 'A'; c <= 'Z'; c++ {
			candidate := baseKey + string(c)
			if len(candidate) > 5 {
				candidate = baseKey[:4] + string(c)
			}
			avail, err := s.workspaceRepo.IsWorkspaceKeyAvailable(ctx, candidate, "", orgIDPtr)
			if err != nil {
				return nil, fmt.Errorf("check workspace key: %w", err)
			}
			if avail {
				req.WorkspaceKey = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("workspace_key %q is already in use and no alternatives available", baseKey)
		}
	}

	// Auto-deduplicate slug if taken.
	baseSlug := req.Slug
	existingWs, err := s.workspaceRepo.GetBySlug(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("check slug availability: %w", err)
	}
	if existingWs != nil {
		slugFound := false
		for i := 2; i <= 99; i++ {
			candidate := fmt.Sprintf("%s-%d", baseSlug, i)
			ex, err := s.workspaceRepo.GetBySlug(ctx, candidate)
			if err != nil {
				return nil, fmt.Errorf("check slug availability: %w", err)
			}
			if ex == nil {
				req.Slug = candidate
				slugFound = true
				break
			}
		}
		if !slugFound {
			return nil, fmt.Errorf("slug %q is already in use and no alternatives available", baseSlug)
		}
	}

	websiteURL, err := normalizeWorkspaceWebsiteURL(req.WebsiteURL)
	if err != nil {
		return nil, err
	}
	if websiteURL != nil && *websiteURL == "" {
		websiteURL = nil
	}

	var orgID *string
	if req.OrganizationID != "" {
		orgID = &req.OrganizationID
	}

	ws, err := s.workspaceRepo.Create(ctx, req.Name, req.Slug, req.WorkspaceKey, ownerID, orgID, req.Description, websiteURL, req.Timezone)
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
func (s *WorkspaceService) Update(ctx context.Context, id string, req model.UpdateWorkspaceRequest, actorID string) (*model.Workspace, error) {
	websiteURL, err := normalizeWorkspaceWebsiteURL(req.WebsiteURL)
	if err != nil {
		return nil, err
	}

	// Handle workspace key change if requested.
	if req.WorkspaceKey != nil {
		newKey := strings.ToUpper(strings.TrimSpace(*req.WorkspaceKey))
		if !workspaceKeyPattern.MatchString(newKey) {
			return nil, fmt.Errorf("workspace_key must be 2-5 uppercase letters")
		}

		current, err := s.workspaceRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get workspace for key change: %w", err)
		}
		if current == nil {
			return nil, fmt.Errorf("workspace not found")
		}

		if newKey != current.WorkspaceKey {
			available, err := s.workspaceRepo.IsWorkspaceKeyAvailable(ctx, newKey, id, current.OrganizationID)
			if err != nil {
				return nil, fmt.Errorf("check workspace key: %w", err)
			}
			if !available {
				return nil, fmt.Errorf("workspace_key %q is already in use", newKey)
			}

			if err := s.workspaceRepo.UpdateWorkspaceKey(ctx, id, current.WorkspaceKey, newKey, actorID); err != nil {
				s.logger.ErrorContext(ctx, "failed to change workspace key", "error", err, "workspace_id", id, "old_key", current.WorkspaceKey, "new_key", newKey)
				return nil, fmt.Errorf("change workspace key: %w", err)
			}
			s.logger.InfoContext(ctx, "workspace key changed", "workspace_id", id, "old_key", current.WorkspaceKey, "new_key", newKey)
		}
	}

	ws, err := s.workspaceRepo.Update(ctx, id, req.Name, req.Description, websiteURL, req.LogoURL, req.Timezone)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to update workspace", "error", err, "workspace_id", id)
		return nil, err
	}
	s.logger.InfoContext(ctx, "workspace updated", "workspace_id", id)
	return ws, nil
}

// GetKeyHistory returns all key changes for a workspace, newest first.
func (s *WorkspaceService) GetKeyHistory(ctx context.Context, workspaceID string) ([]model.WorkspaceKeyHistory, error) {
	return s.workspaceRepo.GetKeyHistory(ctx, workspaceID)
}

func normalizeWorkspaceWebsiteURL(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		empty := ""
		return &empty, nil
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid website URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("website URL must use http or https")
	}

	parsed.Host = strings.ToLower(parsed.Host)
	normalized := parsed.String()
	return &normalized, nil
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
	return s.workspaceRepo.Update(ctx, id, nil, nil, nil, &logoURL, nil)
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
	return s.workspaceRepo.Update(ctx, id, nil, nil, nil, &empty, nil)
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

// ListMemberPresence returns live presence for active workspace members.
func (s *WorkspaceService) ListMemberPresence(ctx context.Context, workspaceID string) ([]model.WorkspaceMemberPresenceStatus, error) {
	statuses, err := resolveSupportTeammatePresenceStatuses(
		ctx,
		s.workspaceRepo,
		s.presence,
		s.statusOverrideRepo,
		workspaceID,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	result := make([]model.WorkspaceMemberPresenceStatus, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, model.WorkspaceMemberPresenceStatus{
			UserID:       status.UserID,
			Status:       status.Status,
			Source:       status.Source,
			ManualStatus: status.ManualStatus,
			LastSeenAt:   status.LastSeenAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UserID < result[j].UserID
	})

	return result, nil
}

// ListAssignableMembers returns joined and pending workspace identities for PM pickers.
func (s *WorkspaceService) ListAssignableMembers(ctx context.Context, workspaceID string) ([]model.AssignableMember, error) {
	return s.workspaceRepo.ListAssignableMembers(ctx, workspaceID)
}

// UpdateMember updates a workspace member role with owner/admin safeguards.
func (s *WorkspaceService) UpdateMember(ctx context.Context, workspaceID, actorID, memberID string, req model.UpdateWorkspaceMemberRequest) error {
	if actorID == "" {
		return fmt.Errorf("actor is required")
	}
	if memberID == "" {
		return fmt.Errorf("member id is required")
	}
	if req.Role != model.RoleOwner && req.Role != model.RoleAdmin && req.Role != model.RoleMember && req.Role != model.RoleViewer {
		return fmt.Errorf("invalid role")
	}

	actorMember, err := s.workspaceRepo.GetMembership(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if actorMember == nil {
		return fmt.Errorf("actor membership not found")
	}
	if actorMember.ID == memberID {
		return fmt.Errorf("cannot change your own role")
	}

	targetMember, err := s.workspaceRepo.GetMembershipByID(ctx, workspaceID, memberID)
	if err != nil {
		return err
	}
	if targetMember == nil || targetMember.Status != model.WorkspaceMemberStatusActive {
		return fmt.Errorf("member not found")
	}

	actorRole := actorMember.Role
	targetRole := targetMember.Role
	if actorRole != model.RoleOwner && actorRole != model.RoleAdmin {
		return fmt.Errorf("only owner or admin can update members")
	}
	if targetRole == model.RoleOwner && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can change an owner's role")
	}
	if targetRole == model.RoleAdmin && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can change an admin's role")
	}
	if req.Role == model.RoleOwner && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can grant ownership")
	}
	if targetRole == model.RoleOwner && req.Role != model.RoleOwner {
		count, err := s.workspaceRepo.CountMembersByRole(ctx, workspaceID, model.RoleOwner)
		if err != nil {
			return err
		}
		if count <= 1 {
			return fmt.Errorf("cannot demote the last owner")
		}
	}

	return s.workspaceRepo.UpdateMemberRole(ctx, workspaceID, memberID, req.Role)
}

// RemoveMember revokes a workspace member with owner/admin safeguards.
func (s *WorkspaceService) RemoveMember(ctx context.Context, workspaceID, actorID, memberID string) error {
	if actorID == "" {
		return fmt.Errorf("actor is required")
	}
	if memberID == "" {
		return fmt.Errorf("member id is required")
	}

	actorMember, err := s.workspaceRepo.GetMembership(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if actorMember == nil {
		return fmt.Errorf("actor membership not found")
	}
	if actorMember.ID == memberID {
		return fmt.Errorf("cannot remove yourself")
	}

	targetMember, err := s.workspaceRepo.GetMembershipByID(ctx, workspaceID, memberID)
	if err != nil {
		return err
	}
	if targetMember == nil || targetMember.Status != model.WorkspaceMemberStatusActive {
		return fmt.Errorf("member not found")
	}

	actorRole := actorMember.Role
	targetRole := targetMember.Role
	if actorRole != model.RoleOwner && actorRole != model.RoleAdmin {
		return fmt.Errorf("only owner or admin can remove members")
	}
	if targetRole == model.RoleOwner {
		return fmt.Errorf("cannot remove an owner")
	}
	if targetRole == model.RoleAdmin && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can remove admins")
	}

	if err := s.workspaceRepo.RemoveMember(ctx, workspaceID, memberID); err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "workspace member removed", "workspace_id", workspaceID, "actor_id", actorID, "member_id", memberID)
	return nil
}
