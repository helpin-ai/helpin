package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type stubOrganizationRoles map[string]string

func (s stubOrganizationRoles) GetMemberRole(_ context.Context, orgID, userID string) (string, error) {
	return s[orgID+"/"+userID], nil
}

func TestWorkspaceCreateRequiresOrganizationMembership(t *testing.T) {
	cases := []struct {
		name    string
		role    string
		allowed bool
	}{
		{"owner", model.RoleOwner, true},
		{"admin", model.RoleAdmin, true},
		{"member", model.RoleMember, true},
		{"viewer", model.RoleViewer, false},
		{"not a member", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, svc := newWorkspaceTestHarness(t)
			svc.SetOrganizationRoles(stubOrganizationRoles{"org-1/owner-1": tc.role})

			ws, err := svc.Create(context.Background(), model.CreateWorkspaceRequest{
				Name: "Org WS", Slug: "org-ws", WorkspaceKey: "ORG", OrganizationID: "org-1",
			}, "owner-1")
			if tc.allowed {
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				if ws.OrganizationID == nil || *ws.OrganizationID != "org-1" {
					t.Fatalf("OrganizationID = %v, want org-1", ws.OrganizationID)
				}
				return
			}
			if !errors.Is(err, ErrWorkspaceOrganizationForbidden) {
				t.Fatalf("Create error = %v, want ErrWorkspaceOrganizationForbidden", err)
			}
			if existing, lookupErr := svc.workspaceRepo.GetBySlug(context.Background(), "org-ws"); lookupErr != nil || existing != nil {
				t.Fatalf("workspace was created despite denial: %v, %v", existing, lookupErr)
			}
		})
	}
}

func TestWorkspaceCreateWithoutOrganizationSkipsMembershipCheck(t *testing.T) {
	_, svc := newWorkspaceTestHarness(t)
	svc.SetOrganizationRoles(stubOrganizationRoles{})

	if _, err := svc.Create(context.Background(), model.CreateWorkspaceRequest{
		Name: "Solo", Slug: "solo", WorkspaceKey: "SOL",
	}, "owner-1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
}
