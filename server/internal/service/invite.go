package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// InviteService handles invitation business logic.
type InviteService struct {
	invitationRepo   *repository.InvitationRepository
	workspaceRepo    *repository.WorkspaceRepository
	organizationRepo *repository.OrganizationRepository
	userRepo         *repository.UserRepository
	settingsRepo     *repository.SettingsRepository
	emailClient      *email.Client
	appBaseURL       string
	jwtManager       *auth.JWTManager
	logger           *slog.Logger
}

// NewInviteService creates a new InviteService.
func NewInviteService(
	invitationRepo *repository.InvitationRepository,
	workspaceRepo *repository.WorkspaceRepository,
	organizationRepo *repository.OrganizationRepository,
	userRepo *repository.UserRepository,
	settingsRepo *repository.SettingsRepository,
	emailClient *email.Client,
	appBaseURL string,
	jwtManager *auth.JWTManager,
) *InviteService {
	return &InviteService{
		invitationRepo:   invitationRepo,
		workspaceRepo:    workspaceRepo,
		organizationRepo: organizationRepo,
		userRepo:         userRepo,
		settingsRepo:     settingsRepo,
		emailClient:      emailClient,
		appBaseURL:       appBaseURL,
		jwtManager:       jwtManager,
		logger:           slog.Default().With("service", "invite"),
	}
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateInvitation creates a new invitation and sends an email.
func (s *InviteService) CreateInvitation(ctx context.Context, req model.CreateInvitationRequest, inviterUserID string) (*model.InvitationResponse, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Validate role
	validRoles := map[string]bool{"admin": true, "manager": true, "member": true, "viewer": true}
	if !validRoles[req.Role] {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Check not already a member
	members, err := s.workspaceRepo.ListMembers(ctx, req.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	for _, m := range members {
		if strings.EqualFold(m.Email, req.Email) {
			return nil, fmt.Errorf("user is already a member of this workspace")
		}
	}

	// Check no pending invite for same email+workspace
	existing, err := s.invitationRepo.GetPendingByEmail(ctx, req.WorkspaceID, req.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("a pending invitation already exists for this email")
	}

	pendingMember, err := s.workspaceRepo.UpsertPendingMember(ctx, req.WorkspaceID, req.Email, req.Role, inviterUserID)
	if err != nil {
		return nil, fmt.Errorf("prepare pending member: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	inv := &model.WorkspaceInvitation{
		WorkspaceID:       req.WorkspaceID,
		WorkspaceMemberID: &pendingMember.ID,
		Email:             req.Email,
		Role:              req.Role,
		Token:             token,
		InvitedBy:         inviterUserID,
		Status:            "pending",
		ExpiresAt:         time.Now().Add(7 * 24 * time.Hour),
	}

	created, err := s.invitationRepo.Create(ctx, inv)
	if err != nil {
		return nil, err
	}

	// Send email
	joinURL := fmt.Sprintf("%s/join/%s", s.appBaseURL, token)

	s.logger.InfoContext(ctx, "invitation created",
		"workspace_id", req.WorkspaceID,
		"email", req.Email,
		"role", req.Role,
		"invitation_id", created.ID,
	)

	if s.emailClient != nil {
		workspace, _ := s.workspaceRepo.GetByID(ctx, req.WorkspaceID)
		inviter, _ := s.userRepo.GetByID(ctx, inviterUserID)
		wsName := req.WorkspaceID
		inviterName := "A team member"
		if workspace != nil {
			wsName = workspace.Name
		}
		if inviter != nil {
			inviterName = inviter.FullName
		}
		if err := s.emailClient.SendInviteEmail(req.Email, inviterName, wsName, joinURL); err != nil {
			s.logger.ErrorContext(ctx, "failed to send invitation email",
				"error", err,
				"workspace_id", req.WorkspaceID,
				"email", req.Email,
			)
		}
	}

	return &model.InvitationResponse{
		ID:                created.ID,
		WorkspaceID:       created.WorkspaceID,
		WorkspaceMemberID: created.WorkspaceMemberID,
		Email:             created.Email,
		Role:              created.Role,
		Status:            created.Status,
		InvitedBy:         created.InvitedBy,
		ExpiresAt:         created.ExpiresAt,
		CreatedAt:         created.CreatedAt,
		JoinURL:           joinURL,
	}, nil
}

// GetInviteInfo returns public info about an invitation for the join page.
func (s *InviteService) GetInviteInfo(ctx context.Context, token string) (*model.InviteInfoResponse, error) {
	details, err := s.invitationRepo.GetByTokenWithDetails(ctx, token)
	if err != nil {
		return nil, err
	}
	if details == nil {
		return nil, fmt.Errorf("invitation not found")
	}

	return &model.InviteInfoResponse{
		WorkspaceName: details.WorkspaceName,
		WorkspaceSlug: details.WorkspaceSlug,
		Email:         details.Email,
		Role:          details.Role,
		InvitedByName: details.InviterName,
		Status:        details.Status,
		Expired:       details.Status == "pending" && time.Now().After(details.ExpiresAt),
	}, nil
}

// AcceptInvitation accepts an invitation and creates workspace membership.
func (s *InviteService) AcceptInvitation(ctx context.Context, token, userID string) error {
	inv, err := s.invitationRepo.GetByToken(ctx, token)
	if err != nil {
		return err
	}
	if inv == nil {
		return fmt.Errorf("invitation not found")
	}
	if inv.Status != "pending" {
		return fmt.Errorf("invitation is no longer pending")
	}
	if time.Now().After(inv.ExpiresAt) {
		return fmt.Errorf("invitation has expired")
	}

	// Verify user's email matches invite email
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	if !strings.EqualFold(user.Email, inv.Email) {
		return fmt.Errorf("your email address does not match the invitation")
	}

	member, err := s.workspaceRepo.ActivatePendingMember(ctx, inv.WorkspaceID, inv.WorkspaceMemberID, userID, user.Email, user.FullName, inv.Role)
	if err != nil {
		return fmt.Errorf("activate member: %w", err)
	}
	if inv.WorkspaceMemberID == nil || *inv.WorkspaceMemberID != member.ID {
		if err := s.invitationRepo.UpdateWorkspaceMemberID(ctx, inv.ID, member.ID); err != nil {
			return fmt.Errorf("link invitation member: %w", err)
		}
	}

	// Ensure user is also a member of the workspace's organization
	s.ensureOrgMembership(ctx, inv.WorkspaceID, userID)

	s.logger.InfoContext(ctx, "invitation accepted",
		"workspace_id", inv.WorkspaceID,
		"email", inv.Email,
		"invitation_id", inv.ID,
		"user_id", userID,
	)

	// Auto-assign teams from preassignments
	preassignments, err := s.settingsRepo.GetInvitationTeamPreassignmentsByInvitation(ctx, inv.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get invitation team preassignments",
			"error", err,
			"invitation_id", inv.ID,
		)
	} else {
		for _, pa := range preassignments {
			if _, err := s.settingsRepo.AddTeamUserMembership(ctx, pa.TeamID, userID, "member"); err != nil {
				s.logger.ErrorContext(ctx, "failed to auto-assign team",
					"error", err,
					"team_id", pa.TeamID,
					"invitation_id", inv.ID,
					"user_id", userID,
				)
			}
		}
	}

	// Update invitation status
	now := time.Now()
	if err := s.invitationRepo.UpdateStatus(ctx, inv.ID, "accepted", &now); err != nil {
		return fmt.Errorf("update invitation status: %w", err)
	}

	return nil
}

// AcceptInvitationWithSignup creates a new user account and accepts the invitation in one step.
func (s *InviteService) AcceptInvitationWithSignup(ctx context.Context, req model.AcceptInvitationWithSignupRequest) (*model.AcceptInvitationWithSignupResponse, error) {
	if req.Token == "" {
		return nil, fmt.Errorf("token is required")
	}
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	if strings.TrimSpace(req.FullName) == "" {
		return nil, fmt.Errorf("full name is required")
	}

	inv, err := s.invitationRepo.GetByToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, fmt.Errorf("invitation not found")
	}
	if inv.Status != "pending" {
		return nil, fmt.Errorf("invitation is no longer pending")
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, fmt.Errorf("invitation has expired")
	}

	details, err := s.invitationRepo.GetByTokenWithDetails(ctx, req.Token)
	if err != nil {
		return nil, fmt.Errorf("get invitation details: %w", err)
	}

	existingUser, err := s.userRepo.GetByEmail(ctx, inv.Email)
	if err != nil {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, fmt.Errorf("an account already exists with this email, please sign in instead")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, inv.Email, hash, strings.TrimSpace(req.FullName))
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	member, err := s.workspaceRepo.ActivatePendingMember(ctx, inv.WorkspaceID, inv.WorkspaceMemberID, user.ID, user.Email, user.FullName, inv.Role)
	if err != nil {
		return nil, fmt.Errorf("activate member: %w", err)
	}
	if inv.WorkspaceMemberID == nil || *inv.WorkspaceMemberID != member.ID {
		if err := s.invitationRepo.UpdateWorkspaceMemberID(ctx, inv.ID, member.ID); err != nil {
			return nil, fmt.Errorf("link invitation member: %w", err)
		}
	}

	// Ensure user is also a member of the workspace's organization
	s.ensureOrgMembership(ctx, inv.WorkspaceID, user.ID)

	s.logger.InfoContext(ctx, "invitation accepted with signup",
		"workspace_id", inv.WorkspaceID,
		"email", inv.Email,
		"invitation_id", inv.ID,
		"user_id", user.ID,
	)

	preassignments, err := s.settingsRepo.GetInvitationTeamPreassignmentsByInvitation(ctx, inv.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get invitation team preassignments",
			"error", err,
			"invitation_id", inv.ID,
		)
	} else {
		for _, pa := range preassignments {
			if _, err := s.settingsRepo.AddTeamUserMembership(ctx, pa.TeamID, user.ID, "member"); err != nil {
				s.logger.ErrorContext(ctx, "failed to auto-assign team",
					"error", err,
					"team_id", pa.TeamID,
					"invitation_id", inv.ID,
					"user_id", user.ID,
				)
			}
		}
	}

	now := time.Now()
	if err := s.invitationRepo.UpdateStatus(ctx, inv.ID, "accepted", &now); err != nil {
		return nil, fmt.Errorf("update invitation status: %w", err)
	}

	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	return &model.AcceptInvitationWithSignupResponse{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		User:          toUserProfile(user),
		WorkspaceSlug: details.WorkspaceSlug,
	}, nil
}

// ListInvitations returns all invitations for a workspace.
func (s *InviteService) ListInvitations(ctx context.Context, workspaceID, userID string) ([]model.InvitationResponse, error) {
	// Verify user is admin/owner
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("get member role: %w", err)
	}
	if role != "owner" && role != "admin" {
		return nil, fmt.Errorf("only admins can list invitations")
	}

	invitations, err := s.invitationRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	result := make([]model.InvitationResponse, len(invitations))
	for i, inv := range invitations {
		resp := model.InvitationResponse{
			ID:                inv.ID,
			WorkspaceID:       inv.WorkspaceID,
			WorkspaceMemberID: inv.WorkspaceMemberID,
			Email:             inv.Email,
			Role:              inv.Role,
			Status:            inv.Status,
			InvitedBy:         inv.InvitedBy,
			ExpiresAt:         inv.ExpiresAt,
			AcceptedAt:        inv.AcceptedAt,
			CreatedAt:         inv.CreatedAt,
		}
		if inv.Status == "pending" {
			resp.JoinURL = fmt.Sprintf("%s/join/%s", s.appBaseURL, inv.Token)
		}
		result[i] = resp
	}
	return result, nil
}

// ResendInvitation resends an invitation email with a new token.
func (s *InviteService) ResendInvitation(ctx context.Context, invitationID, userID string) error {
	inv, err := s.invitationRepo.GetByID(ctx, invitationID)
	if err != nil {
		return err
	}
	if inv == nil {
		return fmt.Errorf("invitation not found")
	}

	// Verify admin
	role, err := s.workspaceRepo.GetMemberRole(ctx, inv.WorkspaceID, userID)
	if err != nil {
		return fmt.Errorf("get member role: %w", err)
	}
	if role != "owner" && role != "admin" {
		return fmt.Errorf("only admins can resend invitations")
	}

	if inv.Status != "pending" {
		return fmt.Errorf("can only resend pending invitations")
	}

	member, err := s.workspaceRepo.UpsertPendingMember(ctx, inv.WorkspaceID, inv.Email, inv.Role, userID)
	if err != nil {
		return fmt.Errorf("prepare pending member: %w", err)
	}
	if inv.WorkspaceMemberID == nil || *inv.WorkspaceMemberID != member.ID {
		if err := s.invitationRepo.UpdateWorkspaceMemberID(ctx, inv.ID, member.ID); err != nil {
			return fmt.Errorf("link invitation member: %w", err)
		}
	}

	// Generate new token and reset expiry
	token, err := generateToken()
	if err != nil {
		return err
	}
	newExpiry := time.Now().Add(7 * 24 * time.Hour)
	if err := s.invitationRepo.UpdateTokenAndExpiry(ctx, invitationID, token, newExpiry); err != nil {
		return err
	}

	// Send email
	joinURL := fmt.Sprintf("%s/join/%s", s.appBaseURL, token)

	s.logger.InfoContext(ctx, "invitation resent",
		"invitation_id", invitationID,
		"workspace_id", inv.WorkspaceID,
		"email", inv.Email,
	)

	if s.emailClient != nil {
		workspace, _ := s.workspaceRepo.GetByID(ctx, inv.WorkspaceID)
		inviter, _ := s.userRepo.GetByID(ctx, userID)
		wsName := inv.WorkspaceID
		inviterName := "A team member"
		if workspace != nil {
			wsName = workspace.Name
		}
		if inviter != nil {
			inviterName = inviter.FullName
		}
		if err := s.emailClient.SendInviteEmail(inv.Email, inviterName, wsName, joinURL); err != nil {
			s.logger.ErrorContext(ctx, "failed to resend invitation email",
				"error", err,
				"invitation_id", invitationID,
				"workspace_id", inv.WorkspaceID,
				"email", inv.Email,
			)
		}
	}

	return nil
}

// RevokeInvitation revokes a pending invitation.
func (s *InviteService) RevokeInvitation(ctx context.Context, invitationID, userID string) error {
	inv, err := s.invitationRepo.GetByID(ctx, invitationID)
	if err != nil {
		return err
	}
	if inv == nil {
		return fmt.Errorf("invitation not found")
	}

	// Verify admin
	role, err := s.workspaceRepo.GetMemberRole(ctx, inv.WorkspaceID, userID)
	if err != nil {
		return fmt.Errorf("get member role: %w", err)
	}
	if role != "owner" && role != "admin" {
		return fmt.Errorf("only admins can revoke invitations")
	}

	if inv.Status != "pending" {
		return fmt.Errorf("can only revoke pending invitations")
	}

	if err := s.invitationRepo.UpdateStatus(ctx, invitationID, "revoked", nil); err != nil {
		return err
	}
	if inv.WorkspaceMemberID != nil {
		if err := s.workspaceRepo.UpdateMemberStatus(ctx, *inv.WorkspaceMemberID, model.WorkspaceMemberStatusRevoked); err != nil {
			return fmt.Errorf("revoke pending workspace member: %w", err)
		}
	}

	s.logger.InfoContext(ctx, "invitation revoked",
		"invitation_id", invitationID,
		"workspace_id", inv.WorkspaceID,
		"email", inv.Email,
	)

	return nil
}

// ensureOrgMembership adds the user to the workspace's organization if they aren't already a member.
func (s *InviteService) ensureOrgMembership(ctx context.Context, workspaceID, userID string) {
	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || ws == nil || ws.OrganizationID == nil {
		return
	}
	if _, err := s.organizationRepo.AddMember(ctx, *ws.OrganizationID, userID, "member"); err != nil {
		s.logger.ErrorContext(ctx, "failed to add user to organization",
			"error", err,
			"organization_id", *ws.OrganizationID,
			"user_id", userID,
		)
	}
}
