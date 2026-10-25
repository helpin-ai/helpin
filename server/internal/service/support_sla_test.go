package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type deniedSLAEntitlements struct{}

func (deniedSLAEntitlements) RequireFeature(context.Context, string, EntitlementFeature) error {
	return &EntitlementError{Feature: string(EntitlementFeatureSLAPolicies), Message: "SLA policies require the Growth plan"}
}

func (deniedSLAEntitlements) RequireLimitUsage(context.Context, string, EntitlementLimit, int64, int64) error {
	return nil
}

func TestSupportSLAServiceSQLite(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID, userID := uuid.NewString(), uuid.NewString()
	seedUser(t, db, userID, "agent@example.com", "Ada Agent", "hash")
	seedWorkspace(t, db, workspaceID, "SLA", "sla-"+workspaceID[:8], userID)
	conversations := repository.NewSupportConversationRepository(db)
	messages := repository.NewSupportMessageRepository(db)
	inbox := NewSupportInboxService(conversations, repository.NewSupportMailboxRepository(db), messages, nil, nil,
		repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, repository.NewUserRepository(db), nil, nil, nil)
	sla := NewSupportSLAService(repository.NewSupportSLARepository(db), inbox)
	inbox.SetSLAService(sla)
	start := time.Now().UTC().Truncate(time.Second)
	sla.now = func() time.Time { return start }

	for name, req := range map[string]model.SupportSLAPolicyRequest{
		"no targets":     {Name: "Empty"},
		"no name":        {FirstResponseMinutes: intPtr(10)},
		"bad priority":   {Name: "X", FirstResponseMinutes: intPtr(10), Priorities: []string{"critical"}},
		"bad channel":    {Name: "X", FirstResponseMinutes: intPtr(10), Channels: []string{"fax"}},
		"bad inbox":      {Name: "X", FirstResponseMinutes: intPtr(10), MailboxIDs: []string{uuid.NewString()}},
		"target too big": {Name: "X", ResolutionMinutes: intPtr(model.SupportSLAPolicyMaxTargetMinutes + 1)},
	} {
		if _, err := sla.CreatePolicy(ctx, workspaceID, userID, req); !errors.Is(err, ErrSupportSLAPolicyInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	first, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "  Email   standard ", Channels: []string{"email", "email"}, FirstResponseMinutes: intPtr(60)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Everything", MailboxIDs: []string{"shared"}, ResolutionMinutes: intPtr(120)})
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "Email standard" || len(first.Channels) != 1 || first.Position != 0 || second.Position != 1 || !first.PauseWhileWaiting {
		t.Fatalf("normalized policies: %+v %+v", first, second)
	}

	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Email", Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "email", CreatedAt: start.Add(time.Second)}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "Hi", CreatedAt: start.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	// SQLite has no triggers: queue the conversation as they would.
	if err := sla.repo.Enqueue(ctx, workspaceID, []string{conversation.ID}, start); err != nil {
		t.Fatal(err)
	}
	if processed, err := sla.ProcessQueue(ctx, 10); err != nil || processed != 1 {
		t.Fatalf("process: %d %v", processed, err)
	}
	listed := []model.SupportConversation{*conversation}
	if err := sla.HydrateConversations(ctx, workspaceID, listed); err != nil {
		t.Fatal(err)
	}
	if listed[0].SLA == nil || listed[0].SLA.PolicyName != "Email standard" || listed[0].SLA.Metrics[0].Metric != model.SupportSLAMetricFirstResponse {
		t.Fatalf("first matching policy not applied: %+v", listed[0].SLA)
	}

	// Moving the second policy first changes which one applies.
	if _, err := sla.ReorderPolicies(ctx, workspaceID, []string{first.ID}); !errors.Is(err, repository.ErrSupportSLAPolicyNotFound) {
		t.Fatalf("partial reorder: %v", err)
	}
	if _, err := sla.ReorderPolicies(ctx, workspaceID, []string{second.ID, first.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := sla.ProcessQueue(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := sla.HydrateConversations(ctx, workspaceID, listed); err != nil {
		t.Fatal(err)
	}
	if listed[0].SLA == nil || listed[0].SLA.PolicyName != "Everything" {
		t.Fatalf("reorder ignored: %+v", listed[0].SLA)
	}

	// Disabling and re-enabling restarts where the policy applies.
	disabled := false
	if _, err := sla.UpdatePolicy(ctx, workspaceID, second.ID, model.SupportSLAPolicyRequest{Name: "Everything", Enabled: &disabled, ResolutionMinutes: intPtr(120)}); err != nil {
		t.Fatal(err)
	}
	sla.now = func() time.Time { return start.Add(time.Hour) }
	enabled := true
	updated, err := sla.UpdatePolicy(ctx, workspaceID, second.ID, model.SupportSLAPolicyRequest{Name: "Everything", Enabled: &enabled, ResolutionMinutes: intPtr(120)})
	if err != nil || !updated.ActiveSince.Equal(start.Add(time.Hour)) {
		t.Fatalf("re-enable: %+v %v", updated, err)
	}
	if _, err := sla.ProcessQueue(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := sla.HydrateConversations(ctx, workspaceID, listed); err != nil {
		t.Fatal(err)
	}
	if listed[0].SLA == nil || listed[0].SLA.PolicyName != "Email standard" {
		t.Fatalf("re-enabled policy applied to an older conversation: %+v", listed[0].SLA)
	}

	if err := sla.DeletePolicy(ctx, workspaceID, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := sla.DeletePolicy(ctx, workspaceID, first.ID); !errors.Is(err, repository.ErrSupportSLAPolicyNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := sla.ProcessQueue(ctx, 10); err != nil {
		t.Fatal(err)
	}
	listed[0].SLA = nil
	if err := sla.HydrateConversations(ctx, workspaceID, listed); err != nil || listed[0].SLA != nil {
		t.Fatalf("SLA kept after its policy was deleted: %+v %v", listed[0].SLA, err)
	}

	sla.SetEntitlementService(deniedSLAEntitlements{})
	var entitlementErr *EntitlementError
	if _, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Paid", FirstResponseMinutes: intPtr(5)}); !errors.As(err, &entitlementErr) {
		t.Fatalf("entitlement not enforced: %v", err)
	}
}
