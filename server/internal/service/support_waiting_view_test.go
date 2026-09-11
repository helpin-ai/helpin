package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportSavedWaitingViewUsesDerivedFilter(t *testing.T) {
	for _, states := range []string{"", "waiting_on_customer"} {
		params := supportInboxViewConversationListParams("ws", "user", model.SupportInboxViewFilters{"nav_filter": "waiting", "states": states, "mailbox_ids": "box", "assignment": "me"})
		if params.Filter != "waiting" || params.Status != "" || len(params.Statuses) != 0 {
			t.Fatalf("saved Waiting params: %+v", params)
		}
	}
}

func TestSupportWaitingView(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedWorkspace(t, db, "waiting-ws", "Waiting", "waiting", "waiting-user")
	repo := repository.NewSupportConversationRepository(db)
	for _, tc := range []struct {
		id, status, sender string
		awaiting           bool
	}{
		{"replied", "open", "user", false},
		{"customer", "open", "customer", true},
		{"stale", "open", "user", true},
		{"ai", "open", "ai", false},
		{"agent", "open", "agent", false},
		{"empty", "open", "", false},
		{"resolved", "resolved", "user", false},
		{"spam", "spam", "user", false},
		{"legacy", "waiting_on_customer", "", false},
	} {
		conv := model.SupportConversation{ID: tc.id, WorkspaceID: "waiting-ws", Subject: tc.id, Status: tc.status, Priority: "medium", Channel: "widget", Source: "widget"}
		if err := repo.Create(ctx, &conv); err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&conv).Updates(map[string]any{"last_public_sender_type": tc.sender, "customer_awaiting_response": tc.awaiting, "support_state_version": 1, "assigned_user_id": "waiting-user"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	params := repository.ConversationRepositoryListParams{ConversationListParams: repository.ConversationListParams{WorkspaceID: "waiting-ws", UserID: "waiting-user", Filter: "waiting", Pagination: model.PMPagination{Page: 1, PerPage: 50}}, Role: model.RoleOwner}
	check := func(want []string) {
		t.Helper()
		rows, _, err := repo.List(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		assertContainsExactly(t, ids, want)
		stats, err := repo.GetUnreadStats(ctx, "waiting-ws", "waiting-user", "", model.RoleOwner, nil)
		if err != nil {
			t.Fatal(err)
		}
		if stats.WaitingTotal != len(want) {
			t.Fatalf("waiting total = %d, want %d", stats.WaitingTotal, len(want))
		}
	}
	check([]string{"replied", "legacy"})
	// Repeated public replies move the thread in/out of Waiting, preserving Open.
	for _, sender := range []string{"customer", "user", "customer", "user"} {
		if err := db.Model(&model.SupportConversation{}).Where("id = ?", "replied").Updates(map[string]any{"last_public_sender_type": sender, "customer_awaiting_response": sender == "customer"}).Error; err != nil {
			t.Fatal(err)
		}
		if sender == "customer" {
			check([]string{"legacy"})
		} else {
			check([]string{"replied", "legacy"})
		}
	}
	var conv model.SupportConversation
	if err := db.First(&conv, "id = ?", "replied").Error; err != nil {
		t.Fatal(err)
	}
	if conv.Status != "open" {
		t.Fatalf("status changed to %s", conv.Status)
	}
	for _, filter := range []string{"inbox", "mine"} {
		params.Filter = filter
		rows, _, err := repo.List(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.ID == "replied" {
				found = true
			}
		}
		if !found {
			t.Fatalf("replied missing from %s", filter)
		}
	}
}
