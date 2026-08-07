package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// OrganizationService handles organization business logic.
type OrganizationService struct {
	orgRepo            *repository.OrganizationRepository
	customerIOIdentity *CustomerIOIdentityService
}

// NewOrganizationService creates a new OrganizationService.
func NewOrganizationService(orgRepo *repository.OrganizationRepository) *OrganizationService {
	return &OrganizationService{orgRepo: orgRepo}
}

func (s *OrganizationService) SetCustomerIOIdentityService(identity *CustomerIOIdentityService) {
	s.customerIOIdentity = identity
}

// requireAdminOrOwner checks that the user has owner or admin role in the org.
func (s *OrganizationService) requireAdminOrOwner(ctx context.Context, orgID, userID string) error {
	role, err := s.orgRepo.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return fmt.Errorf("only owner or admin can perform this action")
	}
	return nil
}

// Create creates an organization and adds the creator as the owner member.
func (s *OrganizationService) Create(ctx context.Context, req model.CreateOrganizationRequest, ownerID string) (*model.OrganizationWithRole, error) {
	if req.Name == "" || req.Slug == "" {
		return nil, fmt.Errorf("name and slug are required")
	}

	slug, err := nextAvailableOrganizationSlug(ctx, s.orgRepo, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("resolve organization slug: %w", err)
	}

	org, err := s.orgRepo.Create(ctx, req.Name, slug, ownerID, req.LogoURL)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}

	_, err = s.orgRepo.AddMember(ctx, org.ID, ownerID, model.RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("add owner as member: %w", err)
	}
	if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncUserByID(ctx, ownerID)
		s.customerIOIdentity.SyncOrganization(ctx, org.ID, ownerID)
	}

	return &model.OrganizationWithRole{
		Organization: *org,
		Role:         model.RoleOwner,
	}, nil
}

// List returns all organizations the user belongs to.
func (s *OrganizationService) List(ctx context.Context, userID string) ([]model.OrganizationWithRole, error) {
	return s.orgRepo.List(ctx, userID)
}

// GetByID returns an organization by its ID. Requires membership.
func (s *OrganizationService) GetByID(ctx context.Context, id, userID string) (*model.Organization, error) {
	role, err := s.orgRepo.GetMemberRole(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, fmt.Errorf("organization not found")
	}
	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, fmt.Errorf("organization not found")
	}
	return org, nil
}

// Update modifies an organization. Only owner or admin can update.
func (s *OrganizationService) Update(ctx context.Context, id, userID string, req model.UpdateOrganizationRequest) (*model.Organization, error) {
	if err := s.requireAdminOrOwner(ctx, id, userID); err != nil {
		return nil, err
	}
	org, err := s.orgRepo.Update(ctx, id, req.Name, req.LogoURL)
	if err != nil {
		return nil, err
	}
	if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncOrganization(ctx, id, userID)
	}
	return org, nil
}

// Delete removes an organization. Only owner can delete.
func (s *OrganizationService) Delete(ctx context.Context, id, userID string) error {
	role, err := s.orgRepo.GetMemberRole(ctx, id, userID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner {
		return fmt.Errorf("only the owner can delete an organization")
	}
	return s.orgRepo.Delete(ctx, id)
}

// ListMembers returns all members of an organization with user details.
func (s *OrganizationService) ListMembers(ctx context.Context, orgID, userID string) ([]model.MemberWithUser, error) {
	role, err := s.orgRepo.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, fmt.Errorf("not a member of this organization")
	}
	return s.orgRepo.ListMembers(ctx, orgID)
}

// ListOwners returns the organization's owner-role members with user details.
// Unscoped by design: used internally (e.g. billing) to resolve who can manage
// billing, not exposed directly to callers.
func (s *OrganizationService) ListOwners(ctx context.Context, orgID string) ([]model.MemberWithUser, error) {
	members, err := s.orgRepo.ListMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	owners := make([]model.MemberWithUser, 0, 1)
	for _, m := range members {
		if m.Role == model.RoleOwner {
			owners = append(owners, m)
		}
	}
	return owners, nil
}

// AddMember adds a user to an organization. Only owner or admin can add.
func (s *OrganizationService) AddMember(ctx context.Context, orgID, actorID string, req model.AddOrgMemberRequest) (*model.OrganizationMember, error) {
	if err := s.requireAdminOrOwner(ctx, orgID, actorID); err != nil {
		return nil, err
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleMember {
		return nil, fmt.Errorf("role must be 'admin' or 'member'")
	}
	member, err := s.orgRepo.AddMember(ctx, orgID, req.UserID, req.Role)
	if err != nil {
		return nil, err
	}
	if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncUserByID(ctx, req.UserID)
		s.customerIOIdentity.SyncOrganization(ctx, orgID, req.UserID)
	}
	return member, nil
}

// UpdateMember updates a member's role. Only owner or admin can update.
func (s *OrganizationService) UpdateMember(ctx context.Context, orgID, actorID, targetUserID string, req model.UpdateOrgMemberRequest) error {
	if actorID == targetUserID {
		return fmt.Errorf("cannot change your own role")
	}
	actorRole, err := s.orgRepo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return err
	}
	if actorRole != model.RoleOwner && actorRole != model.RoleAdmin {
		return fmt.Errorf("only owner or admin can update members")
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleMember && req.Role != model.RoleOwner && req.Role != model.RoleViewer {
		return fmt.Errorf("invalid role")
	}
	targetRole, err := s.orgRepo.GetMemberRole(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}
	// Only owners can manage owners and admins
	if targetRole == model.RoleOwner && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can change an owner's role")
	}
	if targetRole == model.RoleAdmin && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can change an admin's role")
	}
	if req.Role == model.RoleOwner && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can grant ownership")
	}
	// Prevent demoting the last owner
	if targetRole == model.RoleOwner && req.Role != model.RoleOwner {
		count, err := s.orgRepo.CountMembersByRole(ctx, orgID, model.RoleOwner)
		if err != nil {
			return err
		}
		if count <= 1 {
			return fmt.Errorf("cannot demote the last owner")
		}
	}
	if err := s.orgRepo.UpdateMemberRole(ctx, orgID, targetUserID, req.Role); err != nil {
		return err
	}
	if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncOrganization(ctx, orgID, targetUserID)
	}
	return nil
}

// RemoveMember removes a member from an organization. Only owner or admin can remove.
func (s *OrganizationService) RemoveMember(ctx context.Context, orgID, actorID, targetUserID string) error {
	if actorID == targetUserID {
		return fmt.Errorf("cannot remove yourself")
	}
	actorRole, err := s.orgRepo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return err
	}
	if actorRole != model.RoleOwner && actorRole != model.RoleAdmin {
		return fmt.Errorf("only owner or admin can remove members")
	}
	targetRole, err := s.orgRepo.GetMemberRole(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}
	// Only owners can remove owners and admins
	if targetRole == model.RoleOwner {
		return fmt.Errorf("cannot remove an owner")
	}
	if targetRole == model.RoleAdmin && actorRole != model.RoleOwner {
		return fmt.Errorf("only owners can remove admins")
	}
	if err := s.orgRepo.RemoveMember(ctx, orgID, targetUserID); err != nil {
		return err
	}
	if s.customerIOIdentity != nil {
		s.customerIOIdentity.DeleteOrganizationRelationship(ctx, orgID, targetUserID)
		s.customerIOIdentity.SyncOrganization(ctx, orgID, actorID)
	}
	return nil
}

// GetMemberRole returns the role a user has in an organization.
func (s *OrganizationService) GetMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	return s.orgRepo.GetMemberRole(ctx, orgID, userID)
}
