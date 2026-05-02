package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportInboxViewServiceVisibilityAndPermissions(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-custom-views"
	ownerID := "owner-support-custom-views"
	memberID := "member-support-custom-views"
	otherID := "other-support-custom-views"
	seedWorkspace(t, db, workspaceID, "Support Custom Views", "support-custom-views", ownerID)
	seedWorkspaceMember(t, db, "wm-support-custom-member", workspaceID, memberID, "member-custom@example.com", "Member Custom", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-support-custom-other", workspaceID, otherID, "other-custom@example.com", "Other Custom", model.RoleMember)

	service := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), nil)

	privateView, err := service.Create(ctx, workspaceID, memberID, model.RoleMember, model.CreateSupportInboxViewRequest{
		Name:    "My Billing",
		Filters: model.SupportInboxViewFilters{"nav_filter": "inbox", "assignment": "me"},
	})
	if err != nil {
		t.Fatalf("create private view: %v", err)
	}

	sharedView, err := service.Create(ctx, workspaceID, ownerID, model.RoleOwner, model.CreateSupportInboxViewRequest{
		Name:     "AI Needs Review",
		Filters:  model.SupportInboxViewFilters{"nav_filter": "inbox", "ai": "needs_human"},
		IsShared: true,
	})
	if err != nil {
		t.Fatalf("create shared view: %v", err)
	}

	if _, err := service.Create(ctx, workspaceID, memberID, model.RoleMember, model.CreateSupportInboxViewRequest{
		Name:     "Shared From Member",
		Filters:  model.SupportInboxViewFilters{"nav_filter": "waiting"},
		IsShared: true,
	}); err == nil {
		t.Fatal("expected member shared view creation to be forbidden")
	}

	memberViews, err := service.List(ctx, workspaceID, memberID)
	if err != nil {
		t.Fatalf("list member views: %v", err)
	}
	if got, want := idsFromSupportViews(memberViews), []string{sharedView.ID, privateView.ID}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("member views = %v, want %v", got, want)
	}

	otherViews, err := service.List(ctx, workspaceID, otherID)
	if err != nil {
		t.Fatalf("list other views: %v", err)
	}
	if got, want := idsFromSupportViews(otherViews), []string{sharedView.ID}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("other views = %v, want %v", got, want)
	}

	if _, err := service.Update(ctx, privateView.ID, otherID, model.RoleMember, model.UpdateSupportInboxViewRequest{Name: strPtr("Other Rename")}); err == nil {
		t.Fatal("expected private view update by another member to be forbidden")
	}
	if _, err := service.Update(ctx, privateView.ID, ownerID, model.RoleOwner, model.UpdateSupportInboxViewRequest{Name: strPtr("Owner Rename Private")}); err == nil {
		t.Fatal("expected private view update by owner to be forbidden")
	}

	if _, err := service.Update(ctx, sharedView.ID, memberID, model.RoleMember, model.UpdateSupportInboxViewRequest{Name: strPtr("Member Rename")}); err == nil {
		t.Fatal("expected shared view update by non-creator member to be forbidden")
	}

	renamedShared, err := service.Update(ctx, sharedView.ID, ownerID, model.RoleOwner, model.UpdateSupportInboxViewRequest{Name: strPtr("Owner Rename")})
	if err != nil {
		t.Fatalf("owner update shared view: %v", err)
	}
	if renamedShared.Name != "Owner Rename" {
		t.Fatalf("shared view name = %q", renamedShared.Name)
	}

	if err := service.Delete(ctx, sharedView.ID, otherID, model.RoleMember); err == nil {
		t.Fatal("expected shared view delete by non-creator member to be forbidden")
	}
	if err := service.Delete(ctx, sharedView.ID, ownerID, model.RoleOwner); err != nil {
		t.Fatalf("owner delete shared view: %v", err)
	}
}

func idsFromSupportViews(views []model.SupportInboxView) []string {
	ids := make([]string, 0, len(views))
	for _, view := range views {
		ids = append(ids, view.ID)
	}
	return ids
}
