//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPortalSearchPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, _ := seedSupportOpsWorkspace(t, db)
	conversationID := uuid.NewString()
	mustExec(t, db, `INSERT INTO support_conversations (id, workspace_id, display_id, subject, status, channel) VALUES (?, ?, 1, 'Exports are slow', 'open', 'portal')`, conversationID, workspaceID)
	for _, row := range []struct {
		content, metadata string
		internal          bool
	}{
		{"The quarterly totals look fixed now.", `{}`, false},
		{"Private pricing remark.", `{}`, true},
		{"Original text with an internal codename.", `{"merge_public_text":"Merged public wording."}`, false},
	} {
		mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, is_internal, message_type, metadata) VALUES (?, ?, ?, 'user', ?, ?, 'reply', ?)`,
			uuid.NewString(), workspaceID, conversationID, row.content, row.internal, row.metadata)
	}
	repo := repository.NewCustomerPortalRepository(db)
	search := func(query string) []repository.PortalSearchMatch {
		t.Helper()
		matches, err := repo.SearchPortalRequests(ctx, workspaceID, []string{conversationID}, query, 10)
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return matches
	}
	if matches := search("quart tot"); len(matches) != 1 || matches[0].Text != "The quarterly totals look fixed now." {
		t.Fatalf("prefix search = %#v", matches)
	}
	if matches := search("expo"); len(matches) != 1 || matches[0].Text != "" {
		t.Fatalf("subject search = %#v", matches)
	}
	if matches := search("merged wording"); len(matches) != 1 || matches[0].Text != "Merged public wording." {
		t.Fatalf("merged public text search = %#v", matches)
	}
	for _, query := range []string{"pricing", "codename", "'); DROP TABLE x; --"} {
		if matches := search(query); len(matches) != 0 {
			t.Errorf("%q found hidden text: %#v", query, matches)
		}
	}
}
