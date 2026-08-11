package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestOrganizationServiceCreateUsesUniqueSlugWhenRequestedSlugExists(t *testing.T) {
	db := newTestDB(t)
	orgRepo := repository.NewOrganizationRepository(db)
	svc := NewOrganizationService(orgRepo)
	ctx := context.Background()

	first, err := svc.Create(ctx, model.CreateOrganizationRequest{
		Name: "Waqar's Organization",
		Slug: "waqar-s-organization",
	}, "user-1")
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if first.Slug != "waqar-s-organization" {
		t.Fatalf("first slug = %q, want waqar-s-organization", first.Slug)
	}

	second, err := svc.Create(ctx, model.CreateOrganizationRequest{
		Name: "Waqar's Organization",
		Slug: "waqar-s-organization",
	}, "user-2")
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if second.Slug != "waqar-s-organization-2" {
		t.Fatalf("second slug = %q, want waqar-s-organization-2", second.Slug)
	}
	if second.Role != model.RoleOwner {
		t.Fatalf("second role = %q, want owner", second.Role)
	}
}

func TestValidateOrganizationRoleUpdate(t *testing.T) {
	tests := []struct {
		name       string
		actorRole  string
		targetRole string
		newRole    string
		isSelf     bool
		wantError  string
	}{
		{name: "owner promotes member", actorRole: model.RoleOwner, targetRole: model.RoleMember, newRole: model.RoleAdmin},
		{name: "owner demotes admin", actorRole: model.RoleOwner, targetRole: model.RoleAdmin, newRole: model.RoleViewer},
		{name: "admin promotes member", actorRole: model.RoleAdmin, targetRole: model.RoleMember, newRole: model.RoleAdmin},
		{name: "admin cannot demote another admin", actorRole: model.RoleAdmin, targetRole: model.RoleAdmin, newRole: model.RoleMember, wantError: "only organization owners can change an admin"},
		{name: "member cannot manage roles", actorRole: model.RoleMember, targetRole: model.RoleViewer, newRole: model.RoleMember, wantError: "only organization owners and admins"},
		{name: "viewer cannot manage roles", actorRole: model.RoleViewer, targetRole: model.RoleMember, newRole: model.RoleViewer, wantError: "only organization owners and admins"},
		{name: "self change is denied", actorRole: model.RoleAdmin, targetRole: model.RoleAdmin, newRole: model.RoleMember, isSelf: true, wantError: "cannot change your own"},
		{name: "owner target is immutable", actorRole: model.RoleOwner, targetRole: model.RoleOwner, newRole: model.RoleAdmin, wantError: "owners cannot be changed here"},
		{name: "owner role cannot be assigned", actorRole: model.RoleOwner, targetRole: model.RoleAdmin, newRole: model.RoleOwner, wantError: "must be transferred separately"},
		{name: "missing target is rejected", actorRole: model.RoleAdmin, newRole: model.RoleMember, wantError: "member not found"},
		{name: "unknown role is rejected", actorRole: model.RoleAdmin, targetRole: model.RoleMember, newRole: "manager", wantError: "admin', 'member', or 'viewer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOrganizationRoleUpdate(tt.actorRole, tt.targetRole, tt.newRole, tt.isSelf)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validateOrganizationRoleUpdate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("validateOrganizationRoleUpdate() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestOrganizationServiceUpdateMemberPreventsAdminFromManagingAnotherAdmin(t *testing.T) {
	db := newTestDB(t)
	orgRepo := repository.NewOrganizationRepository(db)
	svc := NewOrganizationService(orgRepo)
	ctx := context.Background()

	org, err := svc.Create(ctx, model.CreateOrganizationRequest{Name: "Acme", Slug: "acme"}, "owner-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := orgRepo.AddMember(ctx, org.ID, "admin-1", model.RoleAdmin); err != nil {
		t.Fatalf("AddMember(actor) error = %v", err)
	}
	if _, err := orgRepo.AddMember(ctx, org.ID, "admin-2", model.RoleAdmin); err != nil {
		t.Fatalf("AddMember(target) error = %v", err)
	}

	if err := svc.UpdateMember(ctx, org.ID, "admin-1", "admin-2", model.UpdateOrgMemberRequest{Role: model.RoleViewer}); err == nil {
		t.Fatal("UpdateMember() error = nil, want admin peer-management rejection")
	}
	role, err := orgRepo.GetMemberRole(ctx, org.ID, "admin-2")
	if err != nil {
		t.Fatalf("GetMemberRole() error = %v", err)
	}
	if role != model.RoleAdmin {
		t.Fatalf("target role = %q, want admin", role)
	}
}

func TestOrganizationServiceAddMemberAcceptsViewer(t *testing.T) {
	db := newTestDB(t)
	orgRepo := repository.NewOrganizationRepository(db)
	svc := NewOrganizationService(orgRepo)
	ctx := context.Background()

	org, err := svc.Create(ctx, model.CreateOrganizationRequest{Name: "Viewer Org", Slug: "viewer-org"}, "owner-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	member, err := svc.AddMember(ctx, org.ID, "owner-1", model.AddOrgMemberRequest{UserID: "viewer-1", Role: model.RoleViewer})
	if err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}
	if member.Role != model.RoleViewer {
		t.Fatalf("member role = %q, want viewer", member.Role)
	}
}

func TestOrganizationServiceAddMemberCannotOverwriteExistingRole(t *testing.T) {
	db := newTestDB(t)
	orgRepo := repository.NewOrganizationRepository(db)
	svc := NewOrganizationService(orgRepo)
	ctx := context.Background()

	org, err := svc.Create(ctx, model.CreateOrganizationRequest{Name: "Existing Member Org", Slug: "existing-member-org"}, "owner-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := orgRepo.AddMember(ctx, org.ID, "admin-1", model.RoleAdmin); err != nil {
		t.Fatalf("AddMember(admin) error = %v", err)
	}

	_, err = svc.AddMember(ctx, org.ID, "admin-1", model.AddOrgMemberRequest{UserID: "owner-1", Role: model.RoleViewer})
	if err == nil || !strings.Contains(err.Error(), "already an organization member") {
		t.Fatalf("AddMember(existing owner) error = %v, want existing-member rejection", err)
	}
	role, err := orgRepo.GetMemberRole(ctx, org.ID, "owner-1")
	if err != nil {
		t.Fatalf("GetMemberRole(owner) error = %v", err)
	}
	if role != model.RoleOwner {
		t.Fatalf("owner role = %q, want owner", role)
	}
}

func TestOrganizationServiceTransferOwnership(t *testing.T) {
	db := newTestDB(t)
	orgRepo := repository.NewOrganizationRepository(db)
	svc := NewOrganizationService(orgRepo)
	ctx := context.Background()

	org, err := svc.Create(ctx, model.CreateOrganizationRequest{Name: "Transfer Org", Slug: "transfer-org"}, "owner-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := orgRepo.AddMember(ctx, org.ID, "legacy-owner", model.RoleOwner); err != nil {
		t.Fatalf("AddMember(legacy owner) error = %v", err)
	}
	if _, err := orgRepo.AddMember(ctx, org.ID, "member-1", model.RoleMember); err != nil {
		t.Fatalf("AddMember(new owner) error = %v", err)
	}

	err = svc.TransferOwnership(ctx, org.ID, "owner-1", model.TransferOrganizationOwnershipRequest{NewOwnerID: "member-1"})
	if err != nil {
		t.Fatalf("TransferOwnership() error = %v", err)
	}
	updated, err := orgRepo.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if updated == nil || updated.OwnerID != "member-1" {
		t.Fatalf("organization owner = %v, want member-1", updated)
	}
	for userID, wantRole := range map[string]string{
		"owner-1":      model.RoleAdmin,
		"legacy-owner": model.RoleAdmin,
		"member-1":     model.RoleOwner,
	} {
		role, err := orgRepo.GetMemberRole(ctx, org.ID, userID)
		if err != nil {
			t.Fatalf("GetMemberRole(%s) error = %v", userID, err)
		}
		if role != wantRole {
			t.Fatalf("role for %s = %q, want %q", userID, role, wantRole)
		}
	}
}

func TestOrganizationServiceTransferOwnershipRejectsNonCanonicalOwner(t *testing.T) {
	db := newTestDB(t)
	orgRepo := repository.NewOrganizationRepository(db)
	svc := NewOrganizationService(orgRepo)
	ctx := context.Background()

	org, err := svc.Create(ctx, model.CreateOrganizationRequest{Name: "Transfer Guard Org", Slug: "transfer-guard-org"}, "owner-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := orgRepo.AddMember(ctx, org.ID, "legacy-owner", model.RoleOwner); err != nil {
		t.Fatalf("AddMember(legacy owner) error = %v", err)
	}
	if _, err := orgRepo.AddMember(ctx, org.ID, "member-1", model.RoleMember); err != nil {
		t.Fatalf("AddMember(new owner) error = %v", err)
	}

	err = svc.TransferOwnership(ctx, org.ID, "legacy-owner", model.TransferOrganizationOwnershipRequest{NewOwnerID: "member-1"})
	if err == nil || !strings.Contains(err.Error(), "current organization owner") {
		t.Fatalf("TransferOwnership() error = %v, want canonical-owner rejection", err)
	}
}

func TestOrganizationServiceTransferOwnershipRepairsDriftedOwnerMembership(t *testing.T) {
	tests := []struct {
		name  string
		drift func(context.Context, *repository.OrganizationRepository, string) error
	}{
		{
			name: "wrong role",
			drift: func(ctx context.Context, repo *repository.OrganizationRepository, orgID string) error {
				return repo.UpdateMemberRole(ctx, orgID, "owner-1", model.RoleMember)
			},
		},
		{
			name: "missing membership",
			drift: func(ctx context.Context, repo *repository.OrganizationRepository, orgID string) error {
				return repo.RemoveMember(ctx, orgID, "owner-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			orgRepo := repository.NewOrganizationRepository(db)
			svc := NewOrganizationService(orgRepo)
			ctx := context.Background()

			org, err := svc.Create(ctx, model.CreateOrganizationRequest{Name: "Drift Repair Org", Slug: "drift-repair-org"}, "owner-1")
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if _, err := orgRepo.AddMember(ctx, org.ID, "member-1", model.RoleMember); err != nil {
				t.Fatalf("AddMember(new owner) error = %v", err)
			}
			if err := tt.drift(ctx, orgRepo, org.ID); err != nil {
				t.Fatalf("create owner drift: %v", err)
			}

			if err := svc.TransferOwnership(ctx, org.ID, "owner-1", model.TransferOrganizationOwnershipRequest{NewOwnerID: "member-1"}); err != nil {
				t.Fatalf("TransferOwnership() error = %v", err)
			}
			outgoingRole, err := orgRepo.GetMemberRole(ctx, org.ID, "owner-1")
			if err != nil {
				t.Fatalf("GetMemberRole(outgoing owner) error = %v", err)
			}
			if outgoingRole != model.RoleAdmin {
				t.Fatalf("outgoing owner role = %q, want admin", outgoingRole)
			}
			newOwnerRole, err := orgRepo.GetMemberRole(ctx, org.ID, "member-1")
			if err != nil {
				t.Fatalf("GetMemberRole(new owner) error = %v", err)
			}
			if newOwnerRole != model.RoleOwner {
				t.Fatalf("new owner role = %q, want owner", newOwnerRole)
			}
		})
	}
}
