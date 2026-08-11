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
		{name: "admin demotes another admin", actorRole: model.RoleAdmin, targetRole: model.RoleAdmin, newRole: model.RoleMember},
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

func TestOrganizationServiceUpdateMemberAllowsAdminToManageAnotherAdmin(t *testing.T) {
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

	if err := svc.UpdateMember(ctx, org.ID, "admin-1", "admin-2", model.UpdateOrgMemberRequest{Role: model.RoleViewer}); err != nil {
		t.Fatalf("UpdateMember() error = %v", err)
	}
	role, err := orgRepo.GetMemberRole(ctx, org.ID, "admin-2")
	if err != nil {
		t.Fatalf("GetMemberRole() error = %v", err)
	}
	if role != model.RoleViewer {
		t.Fatalf("target role = %q, want viewer", role)
	}
}
