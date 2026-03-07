package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// InviteService handles invitation business logic.
type InviteService struct {
	invitationRepo *repository.InvitationRepository
	workspaceRepo  *repository.WorkspaceRepository
	userRepo       *repository.UserRepository
	settingsRepo   *repository.SettingsRepository
	emailClient    *email.Client
	appBaseURL     string
}

// NewInviteService creates a new InviteService.
func NewInviteService(
	invitationRepo *repository.InvitationRepository,
	workspaceRepo *repository.WorkspaceRepository,
	userRepo *repository.UserRepository,
	settingsRepo *repository.SettingsRepository,
	emailClient *email.Client,
	appBaseURL string,
) *InviteService {
	return &InviteService{
		invitationRepo: invitationRepo,
		workspaceRepo:  workspaceRepo,
		userRepo:       userRepo,
		settingsRepo:   settingsRepo,
		emailClient:    emailClient,
		appBaseURL:     appBaseURL,
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
	log.Printf("Invitation join URL: %s", joinURL)

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
			log.Printf("Warning: failed to send invitation email: %v", err)
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

	// Auto-assign teams from preassignments
	preassignments, err := s.settingsRepo.GetInvitationTeamPreassignmentsByInvitation(ctx, inv.ID)
	if err != nil {
		log.Printf("Warning: failed to get invitation team preassignments: %v", err)
	} else {
		for _, pa := range preassignments {
			if _, err := s.settingsRepo.AddTeamUserMembership(ctx, pa.TeamID, userID, "member"); err != nil {
				log.Printf("Warning: failed to auto-assign team %s for invitation %s: %v", pa.TeamID, inv.ID, err)
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
	log.Printf("Resent invitation join URL: %s", joinURL)

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
			log.Printf("Warning: failed to resend invitation email: %v", err)
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
	return nil
}
