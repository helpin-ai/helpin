package service

import (
	"context"
	"testing"
	"time"

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

	service := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), nil, nil)

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

	if _, err := service.Update(ctx, workspaceID, privateView.ID, otherID, model.RoleMember, model.UpdateSupportInboxViewRequest{Name: strPtr("Other Rename")}); err == nil {
		t.Fatal("expected private view update by another member to be forbidden")
	}
	if _, err := service.Update(ctx, workspaceID, privateView.ID, ownerID, model.RoleOwner, model.UpdateSupportInboxViewRequest{Name: strPtr("Owner Rename Private")}); err == nil {
		t.Fatal("expected private view update by owner to be forbidden")
	}

	if _, err := service.Update(ctx, workspaceID, sharedView.ID, memberID, model.RoleMember, model.UpdateSupportInboxViewRequest{Name: strPtr("Member Rename")}); err == nil {
		t.Fatal("expected shared view update by non-creator member to be forbidden")
	}

	renamedShared, err := service.Update(ctx, workspaceID, sharedView.ID, ownerID, model.RoleOwner, model.UpdateSupportInboxViewRequest{Name: strPtr("Owner Rename")})
	if err != nil {
		t.Fatalf("owner update shared view: %v", err)
	}
	if renamedShared.Name != "Owner Rename" {
		t.Fatalf("shared view name = %q", renamedShared.Name)
	}

	if err := service.Delete(ctx, workspaceID, sharedView.ID, otherID, model.RoleMember); err == nil {
		t.Fatal("expected shared view delete by non-creator member to be forbidden")
	}
	if err := service.Delete(ctx, workspaceID, sharedView.ID, ownerID, model.RoleOwner); err != nil {
		t.Fatalf("owner delete shared view: %v", err)
	}
}

func TestSupportInboxViewServiceScopesUpdateAndDeleteToWorkspace(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-custom-views-scope-a"
	otherWorkspaceID := "ws-support-custom-views-scope-b"
	ownerID := "owner-support-custom-views-scope"
	seedWorkspace(t, db, workspaceID, "Support Custom Views Scope A", "support-custom-views-scope-a", ownerID)
	seedWorkspace(t, db, otherWorkspaceID, "Support Custom Views Scope B", "support-custom-views-scope-b", ownerID)

	service := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), nil, nil)

	view, err := service.Create(ctx, otherWorkspaceID, ownerID, model.RoleOwner, model.CreateSupportInboxViewRequest{
		Name:     "Other Workspace View",
		Filters:  model.SupportInboxViewFilters{"nav_filter": "inbox"},
		IsShared: true,
	})
	if err != nil {
		t.Fatalf("create other workspace view: %v", err)
	}

	if _, err := service.Update(ctx, workspaceID, view.ID, ownerID, model.RoleOwner, model.UpdateSupportInboxViewRequest{Name: strPtr("Cross Workspace Rename")}); err == nil {
		t.Fatal("expected cross-workspace view update to fail")
	}
	if err := service.Delete(ctx, workspaceID, view.ID, ownerID, model.RoleOwner); err == nil {
		t.Fatal("expected cross-workspace view delete to fail")
	}

	views, err := service.List(ctx, otherWorkspaceID, ownerID)
	if err != nil {
		t.Fatalf("list other workspace views: %v", err)
	}
	if got := idsFromSupportViews(views); len(got) != 1 || got[0] != view.ID {
		t.Fatalf("other workspace views after failed cross-workspace mutations = %v, want [%s]", got, view.ID)
	}
}

func TestSupportInboxViewServiceUpsertsBuiltinViewsInViewsTable(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-builtin-views"
	userID := "user-support-builtin-views"
	otherUserID := "other-support-builtin-views"
	seedWorkspace(t, db, workspaceID, "Support Builtin Views", "support-builtin-views", userID)
	seedWorkspaceMember(t, db, "wm-support-builtin-user", workspaceID, userID, "user-builtin@example.com", "Builtin User", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-support-builtin-other", workspaceID, otherUserID, "other-builtin@example.com", "Other Builtin", model.RoleMember)
	service := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), nil, nil)

	view, err := service.UpsertBuiltinView(ctx, workspaceID, userID, model.UpdateSupportInboxBuiltinViewRequest{
		ViewKey: "nav:inbox",
		Filters: model.SupportInboxViewFilters{
			"nav_filter":  "inbox",
			"states":      "open,waiting_on_customer",
			"mailbox_ids": "all",
		},
	})
	if err != nil {
		t.Fatalf("upsert builtin view: %v", err)
	}
	if view.ViewType != model.SupportInboxViewTypeDefault || view.ViewKey == nil || *view.ViewKey != "nav:inbox" || view.Filters["mailbox_ids"] != "all" {
		t.Fatalf("unexpected builtin view: %#v", view)
	}

	updated, err := service.UpsertBuiltinView(ctx, workspaceID, userID, model.UpdateSupportInboxBuiltinViewRequest{
		ViewKey: "nav:inbox",
		Filters: model.SupportInboxViewFilters{
			"nav_filter": "inbox",
			"states":     "open",
		},
	})
	if err != nil {
		t.Fatalf("update builtin view: %v", err)
	}
	if updated.ID != view.ID {
		t.Fatalf("expected upsert to update existing view, got %s then %s", view.ID, updated.ID)
	}

	builtinViews, err := service.ListBuiltinViews(ctx, workspaceID, userID)
	if err != nil {
		t.Fatalf("list builtin views: %v", err)
	}
	if len(builtinViews) != 1 || builtinViews[0].Filters["states"] != "open" {
		t.Fatalf("unexpected listed builtin views: %#v", builtinViews)
	}

	customViews, err := service.List(ctx, workspaceID, userID)
	if err != nil {
		t.Fatalf("list custom views: %v", err)
	}
	if len(customViews) != 0 {
		t.Fatalf("expected builtin views to stay out of custom view list, got %#v", customViews)
	}
	if _, err := service.Update(ctx, workspaceID, view.ID, userID, model.RoleMember, model.UpdateSupportInboxViewRequest{Name: strPtr("Rename Builtin")}); err == nil {
		t.Fatalf("expected generic custom view update to reject builtin views")
	}
	if err := service.Delete(ctx, workspaceID, view.ID, userID, model.RoleMember); err == nil {
		t.Fatalf("expected generic custom view delete to reject builtin views")
	}

	otherViews, err := service.ListBuiltinViews(ctx, workspaceID, otherUserID)
	if err != nil {
		t.Fatalf("list other builtin views: %v", err)
	}
	if len(otherViews) != 0 {
		t.Fatalf("expected builtin views to be user-scoped, got %#v", otherViews)
	}
}

func TestSupportInboxViewServiceRejectsInvalidBuiltinViewKeys(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-builtin-invalid"
	userID := "user-support-builtin-invalid"
	seedWorkspace(t, db, workspaceID, "Support Builtin Invalid", "support-builtin-invalid", userID)
	seedWorkspaceMember(t, db, "wm-support-builtin-invalid", workspaceID, userID, "invalid-builtin@example.com", "Invalid Builtin", model.RoleMember)
	service := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), nil, nil)

	_, err := service.UpsertBuiltinView(ctx, workspaceID, userID, model.UpdateSupportInboxBuiltinViewRequest{
		ViewKey: "bad:key",
		Filters: model.SupportInboxViewFilters{
			"states": "open",
		},
	})
	if err == nil {
		t.Fatalf("expected invalid preference key to be rejected")
	}
}

func TestSupportInboxViewServiceCountsCustomViewsFromSavedFilters(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-custom-view-counts"
	userID := "user-support-custom-view-counts"
	seedWorkspace(t, db, workspaceID, "Support Custom View Counts", "support-custom-view-counts", userID)
	seedWorkspaceMember(t, db, "wm-support-custom-view-counts", workspaceID, userID, "counts@example.com", "Counts User", model.RoleOwner)
	conversationRepo := repository.NewSupportConversationRepository(db)
	viewService := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), conversationRepo, nil)
	now := time.Now()

	openUnread := &model.SupportConversation{
		WorkspaceID:    workspaceID,
		Subject:        "Open unread",
		Status:         model.SupportConversationStatusOpen,
		AssignedUserID: &userID,
		TeamLastSeenAt: ptrTime(now.Add(-2 * time.Hour)),
	}
	if err := conversationRepo.Create(ctx, openUnread); err != nil {
		t.Fatalf("create open unread conversation: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
		"msg-custom-view-counts-unread", workspaceID, openUnread.ID, "Unread reply", now.Add(-time.Hour), now.Add(-time.Hour),
	).Error; err != nil {
		t.Fatalf("seed unread message: %v", err)
	}

	openRead := &model.SupportConversation{
		WorkspaceID:    workspaceID,
		Subject:        "Open read",
		Status:         model.SupportConversationStatusOpen,
		AssignedUserID: &userID,
		TeamLastSeenAt: &now,
	}
	if err := conversationRepo.Create(ctx, openRead); err != nil {
		t.Fatalf("create open read conversation: %v", err)
	}

	otherUserID := "other-support-custom-view-counts"
	otherConversation := &model.SupportConversation{
		WorkspaceID:    workspaceID,
		Subject:        "Other user",
		Status:         model.SupportConversationStatusOpen,
		AssignedUserID: &otherUserID,
		TeamLastSeenAt: ptrTime(now.Add(-2 * time.Hour)),
	}
	if err := conversationRepo.Create(ctx, otherConversation); err != nil {
		t.Fatalf("create other conversation: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
		"msg-custom-view-counts-other", workspaceID, otherConversation.ID, "Other unread reply", now.Add(-time.Hour), now.Add(-time.Hour),
	).Error; err != nil {
		t.Fatalf("seed other unread message: %v", err)
	}

	view, err := viewService.Create(ctx, workspaceID, userID, model.RoleOwner, model.CreateSupportInboxViewRequest{
		Name: "My open work",
		Filters: model.SupportInboxViewFilters{
			"nav_filter":  "inbox",
			"states":      "open",
			"assignment":  "me",
			"mailbox_ids": "all",
		},
		IsShared: false,
	})
	if err != nil {
		t.Fatalf("create custom view: %v", err)
	}

	counts, err := viewService.ListCounts(ctx, workspaceID, userID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("list custom view counts: %v", err)
	}
	if len(counts) != 1 {
		t.Fatalf("expected one custom view count, got %#v", counts)
	}
	if counts[0].ViewID != view.ID || counts[0].TotalCount != 2 || counts[0].UnreadCount != 1 {
		t.Fatalf("unexpected custom view count: %#v", counts[0])
	}
}

func TestSupportInboxViewServiceCountsTreatsReorderedDefaultStatesAsDefault(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-custom-view-reordered-states"
	userID := "user-support-custom-view-reordered-states"
	seedWorkspace(t, db, workspaceID, "Support Custom View Reordered States", "support-custom-view-reordered-states", userID)
	seedWorkspaceMember(t, db, "wm-support-custom-view-reordered-states", workspaceID, userID, "reordered@example.com", "Reordered User", model.RoleOwner)
	conversationRepo := repository.NewSupportConversationRepository(db)
	viewService := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), conversationRepo, nil)

	for _, subject := range []string{"Human open", "Human waiting"} {
		status := model.SupportConversationStatusOpen
		if subject == "Human waiting" {
			status = model.SupportConversationStatusWaitingOnCustomer
		}
		if err := conversationRepo.Create(ctx, &model.SupportConversation{
			WorkspaceID:    workspaceID,
			Subject:        subject,
			Status:         status,
			AssignedUserID: &userID,
		}); err != nil {
			t.Fatalf("create %s conversation: %v", subject, err)
		}
	}

	flowState := model.SupportConversationFlowStateAIHandling
	if err := conversationRepo.Create(ctx, &model.SupportConversation{
		WorkspaceID:    workspaceID,
		Subject:        "AI active",
		Status:         model.SupportConversationStatusOpen,
		FlowState:      &flowState,
		AssignedUserID: &userID,
	}); err != nil {
		t.Fatalf("create ai active conversation: %v", err)
	}

	view, err := viewService.Create(ctx, workspaceID, userID, model.RoleOwner, model.CreateSupportInboxViewRequest{
		Name: "Inbox default states, reordered",
		Filters: model.SupportInboxViewFilters{
			"nav_filter":  "inbox",
			"states":      "waiting_on_customer,open",
			"assignment":  "me",
			"mailbox_ids": "all",
		},
	})
	if err != nil {
		t.Fatalf("create custom view: %v", err)
	}

	counts, err := viewService.ListCounts(ctx, workspaceID, userID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("list custom view counts: %v", err)
	}
	if len(counts) != 1 {
		t.Fatalf("expected one custom view count, got %#v", counts)
	}
	if counts[0].ViewID != view.ID || counts[0].TotalCount != 2 {
		t.Fatalf("expected reordered default states to keep inbox semantics, got %#v", counts[0])
	}
}

func TestSupportInboxViewServiceCountsSupportsNoAIStateFilter(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-custom-view-ai-none"
	userID := "user-support-custom-view-ai-none"
	seedWorkspace(t, db, workspaceID, "Support Custom View AI None", "support-custom-view-ai-none", userID)
	seedWorkspaceMember(t, db, "wm-support-custom-view-ai-none", workspaceID, userID, "ai-none@example.com", "AI None User", model.RoleOwner)
	conversationRepo := repository.NewSupportConversationRepository(db)
	viewService := NewSupportInboxViewService(repository.NewSupportInboxViewRepository(db), conversationRepo, nil)

	if err := conversationRepo.Create(ctx, &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Normal human work",
		Status:      model.SupportConversationStatusOpen,
	}); err != nil {
		t.Fatalf("create normal conversation: %v", err)
	}

	flowState := model.SupportConversationFlowStateAIHandling
	if err := conversationRepo.Create(ctx, &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "AI active work",
		Status:      model.SupportConversationStatusOpen,
		FlowState:   &flowState,
	}); err != nil {
		t.Fatalf("create ai active conversation: %v", err)
	}

	escalated := "escalated"
	if err := conversationRepo.Create(ctx, &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "AI handoff work",
		Status:      model.SupportConversationStatusOpen,
		AIState:     &escalated,
	}); err != nil {
		t.Fatalf("create ai handoff conversation: %v", err)
	}

	resolvedFlowState := model.SupportConversationFlowStateResolvedByAI
	if err := conversationRepo.Create(ctx, &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "AI resolved work",
		Status:      model.SupportConversationStatusResolved,
		FlowState:   &resolvedFlowState,
	}); err != nil {
		t.Fatalf("create ai resolved conversation: %v", err)
	}

	view, err := viewService.Create(ctx, workspaceID, userID, model.RoleOwner, model.CreateSupportInboxViewRequest{
		Name: "No AI state",
		Filters: model.SupportInboxViewFilters{
			"nav_filter":  "inbox",
			"states":      "open,waiting_on_customer",
			"mailbox_ids": "all",
			"ai":          "none",
		},
	})
	if err != nil {
		t.Fatalf("create ai none custom view: %v", err)
	}

	counts, err := viewService.ListCounts(ctx, workspaceID, userID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("list custom view counts: %v", err)
	}
	if len(counts) != 1 {
		t.Fatalf("expected one custom view count, got %#v", counts)
	}
	if counts[0].ViewID != view.ID || counts[0].TotalCount != 1 {
		t.Fatalf("expected ai=none count to include only normal conversation, got %#v", counts[0])
	}
}

func idsFromSupportViews(views []model.SupportInboxView) []string {
	ids := make([]string, 0, len(views))
	for _, view := range views {
		ids = append(ids, view.ID)
	}
	return ids
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
