package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

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
	// No policy applies any more: open targets end, the record stays.
	listed[0].SLA = nil
	if err := sla.HydrateConversations(ctx, workspaceID, listed); err != nil || listed[0].SLA == nil ||
		listed[0].SLA.NextDueAt != nil || listed[0].SLA.Breached || listed[0].SLA.Metrics[0].State != model.SupportSLAStateCancelled {
		t.Fatalf("detached SLA: %+v %v", listed[0].SLA, err)
	}

	sla.SetEntitlementService(deniedSLAEntitlements{})
	var entitlementErr *EntitlementError
	if _, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Paid", FirstResponseMinutes: intPtr(5)}); !errors.As(err, &entitlementErr) {
		t.Fatalf("entitlement not enforced: %v", err)
	}
}

func newSLATestService(t *testing.T) (*SupportSLAService, *gorm.DB, string, string, time.Time) {
	t.Helper()
	db := newTestDB(t)
	workspaceID, userID := uuid.NewString(), uuid.NewString()
	seedUser(t, db, userID, "agent@example.com", "Ada Agent", "hash")
	seedWorkspace(t, db, workspaceID, "SLA", "sla-"+workspaceID[:8], userID)
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil,
		repository.NewUserRepository(db), nil, nil, nil)
	sla := NewSupportSLAService(repository.NewSupportSLARepository(db), inbox)
	inbox.SetSLAService(sla)
	start := time.Now().UTC().Truncate(time.Second).Add(-24 * time.Hour)
	sla.now = func() time.Time { return start }
	return sla, db, workspaceID, userID, start
}

func slaRecord(t *testing.T, db *gorm.DB, conversationID string) model.SupportConversationSLARecord {
	t.Helper()
	var record model.SupportConversationSLARecord
	if err := db.Where("conversation_id = ?", conversationID).Take(&record).Error; err != nil {
		t.Fatalf("load SLA record: %v", err)
	}
	return record
}

func processSLAAt(t *testing.T, sla *SupportSLAService, workspaceID, conversationID string, at time.Time) {
	t.Helper()
	sla.now = func() time.Time { return at }
	if err := sla.repo.Enqueue(context.Background(), workspaceID, []string{conversationID}, at); err != nil {
		t.Fatal(err)
	}
	if _, err := sla.ProcessQueue(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
}

func TestSupportSLAPolicyEditsKeepHistory(t *testing.T) {
	sla, db, workspaceID, userID, start := newSLATestService(t)
	ctx := context.Background()
	policy, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(240), ResolutionMinutes: intPtr(600)})
	if err != nil {
		t.Fatal(err)
	}
	messages := repository.NewSupportMessageRepository(db)
	newConversation := func(displayID int) string {
		conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: displayID, Subject: "Help",
			Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "widget", CreatedAt: start.Add(time.Second)}
		if err := db.Create(conversation).Error; err != nil {
			t.Fatal(err)
		}
		for _, m := range []struct {
			sender string
			at     time.Duration
		}{{"customer", time.Second}, {"user", 2 * time.Hour}} {
			if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: m.sender,
				MessageType: "reply", Content: "Hi", CreatedAt: start.Add(m.at)}); err != nil {
				t.Fatal(err)
			}
		}
		return conversation.ID
	}
	openID, resolvedID := newConversation(1), newConversation(2)
	processSLAAt(t, sla, workspaceID, openID, start.Add(3*time.Hour))
	processSLAAt(t, sla, workspaceID, resolvedID, start.Add(3*time.Hour))
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", resolvedID).
		Updates(map[string]any{"status": model.SupportConversationStatusResolved, "resolved_at": start.Add(3 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	processSLAAt(t, sla, workspaceID, resolvedID, start.Add(3*time.Hour))
	if got := slaRecord(t, db, resolvedID); got.FirstResponseState != model.SupportSLAStateAchieved || got.ResolutionState != model.SupportSLAStateAchieved {
		t.Fatalf("resolved in time: %+v", got)
	}

	// Tightening the targets judges neither conversation again.
	if _, err := sla.UpdatePolicy(ctx, workspaceID, policy.ID, model.SupportSLAPolicyRequest{Name: "Standard", FirstResponseMinutes: intPtr(60), ResolutionMinutes: intPtr(90)}); err != nil {
		t.Fatal(err)
	}
	processSLAAt(t, sla, workspaceID, openID, start.Add(4*time.Hour))
	processSLAAt(t, sla, workspaceID, resolvedID, start.Add(4*time.Hour))
	open, resolved := slaRecord(t, db, openID), slaRecord(t, db, resolvedID)
	if open.FirstResponseState != model.SupportSLAStateAchieved || *open.TargetFirstResponseMinutes != 240 ||
		open.ResolutionState != model.SupportSLAStatePending || !open.ResolutionDueAt.Equal(start.Add(time.Second+10*time.Hour)) {
		t.Fatalf("policy edit re-judged an open conversation: %+v", open)
	}
	if resolved.FirstResponseState != model.SupportSLAStateAchieved || resolved.ResolutionState != model.SupportSLAStateAchieved {
		t.Fatalf("policy edit re-judged a resolved conversation: %+v", resolved)
	}

	// Disabling stops the open timers and keeps every recorded result.
	disabled := false
	if _, err := sla.UpdatePolicy(ctx, workspaceID, policy.ID, model.SupportSLAPolicyRequest{Name: "Standard", Enabled: &disabled, FirstResponseMinutes: intPtr(60)}); err != nil {
		t.Fatal(err)
	}
	sla.now = func() time.Time { return start.Add(5 * time.Hour) }
	if _, err := sla.ProcessQueue(ctx, 10); err != nil {
		t.Fatal(err)
	}
	processSLAAt(t, sla, workspaceID, resolvedID, start.Add(5*time.Hour))
	open, resolved = slaRecord(t, db, openID), slaRecord(t, db, resolvedID)
	if open.DetachedAt == nil || open.FirstResponseState != model.SupportSLAStateAchieved || open.ResolutionState != model.SupportSLAStateCancelled || open.NextDueAt != nil {
		t.Fatalf("disabled policy on an open conversation: %+v", open)
	}
	if resolved.DetachedAt != nil || resolved.ResolutionState != model.SupportSLAStateAchieved {
		t.Fatalf("disabled policy changed a resolved conversation: %+v", resolved)
	}
	var breaches int64
	if err := db.Model(&model.SupportMessage{}).Where("workspace_id = ? AND system_event_type = ?", workspaceID, model.SystemEventSLABreached).
		Count(&breaches).Error; err != nil || breaches != 0 {
		t.Fatalf("policy changes announced breaches: %d %v", breaches, err)
	}
}

func TestSupportSLAWorkLease(t *testing.T) {
	sla, _, workspaceID, _, start := newSLATestService(t)
	ctx := context.Background()
	repo := sla.repo
	a, b, later := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := repo.Enqueue(ctx, workspaceID, []string{a, b}, start); err != nil {
		t.Fatal(err)
	}
	if err := repo.Enqueue(ctx, workspaceID, []string{later}, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.ClaimWork(ctx, start, start.Add(supportSLAWorkLease), 10)
	if err != nil || len(claimed) != 2 {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if again, err := repo.ClaimWork(ctx, start.Add(time.Second), start.Add(supportSLAWorkLease), 10); err != nil || len(again) != 0 {
		t.Fatalf("leased or future rows claimed twice: %+v %v", again, err)
	}
	// One evaluation finishes; the other conversation changes meanwhile.
	byID := map[string]model.SupportSLAWork{}
	for _, item := range claimed {
		byID[item.ConversationID] = item
	}
	if err := repo.Enqueue(ctx, workspaceID, []string{b}, start.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a, b} {
		if err := repo.CompleteWork(ctx, byID[id]); err != nil {
			t.Fatal(err)
		}
	}
	rest, err := repo.ClaimWork(ctx, start.Add(time.Minute), start.Add(time.Minute+supportSLAWorkLease), 10)
	if err != nil || len(rest) != 2 {
		t.Fatalf("re-queued and due rows: %+v %v", rest, err)
	}
	ids := map[string]bool{rest[0].ConversationID: true, rest[1].ConversationID: true}
	if !ids[b] || !ids[later] {
		t.Fatalf("expected the changed and the now-due conversations, got %+v", rest)
	}
	// A lease that runs out returns the row.
	if expired, err := repo.ClaimWork(ctx, start.Add(time.Minute+supportSLAWorkLease), start.Add(time.Hour), 10); err != nil || len(expired) != 2 {
		t.Fatalf("expired leases: %+v %v", expired, err)
	}
}

func TestSupportSLAFalseSettingsPersist(t *testing.T) {
	sla, db, workspaceID, userID, start := newSLATestService(t)
	ctx := context.Background()
	disabled, noPause := false, false
	policy, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Off", Enabled: &disabled, PauseWhileWaiting: &noPause, ResolutionMinutes: intPtr(60)})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := sla.repo.GetPolicy(ctx, workspaceID, policy.ID)
	if err != nil || stored.Enabled || stored.PauseWhileWaiting {
		t.Fatalf("false settings saved as true: %+v %v", stored, err)
	}
	record := model.SupportConversationSLARecord{ConversationID: uuid.NewString(), WorkspaceID: workspaceID, PolicyID: &policy.ID, TargetResolutionMinutes: intPtr(60)}
	for i := 0; i < 2; i++ {
		if err := sla.repo.SaveRecord(ctx, &record); err != nil {
			t.Fatal(err)
		}
		if got := slaRecord(t, db, record.ConversationID); got.TargetPauseWhileWaiting {
			t.Fatalf("save %d stored pause_while_waiting as true", i+1)
		}
	}
	_ = start
}

func TestSupportSLABroadenedConditionsDoNotBackdate(t *testing.T) {
	sla, db, workspaceID, userID, start := newSLATestService(t)
	ctx := context.Background()
	policy, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Urgent", Priorities: []string{"urgent"}, FirstResponseMinutes: intPtr(30)})
	if err != nil {
		t.Fatal(err)
	}
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Help",
		Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "widget", CreatedAt: start.Add(time.Second)}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.NewSupportMessageRepository(db).Create(ctx, &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID,
		SenderType: "customer", MessageType: "reply", Content: "Hi", CreatedAt: start.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	processSLAAt(t, sla, workspaceID, conversation.ID, start.Add(time.Minute))
	var count int64
	db.Model(&model.SupportConversationSLARecord{}).Where("conversation_id = ?", conversation.ID).Count(&count)
	if count != 0 {
		t.Fatal("a medium conversation matched the urgent policy")
	}
	// Two hours later the policy is widened to every priority.
	changedAt := start.Add(2 * time.Hour)
	sla.now = func() time.Time { return changedAt }
	if _, err := sla.UpdatePolicy(ctx, workspaceID, policy.ID, model.SupportSLAPolicyRequest{Name: "Urgent", FirstResponseMinutes: intPtr(30)}); err != nil {
		t.Fatal(err)
	}
	processSLAAt(t, sla, workspaceID, conversation.ID, changedAt.Add(time.Minute))
	record := slaRecord(t, db, conversation.ID)
	if record.Breached || record.FirstResponseState != model.SupportSLAStatePending || !record.FirstResponseDueAt.Equal(changedAt.Add(30*time.Minute)) {
		t.Fatalf("widened policy back-dated the target: %+v", record)
	}
}

func TestSupportSLAInboxFilters(t *testing.T) {
	_, db, workspaceID, _, _ := newSLATestService(t)
	now := time.Now().UTC()
	ids := map[string]string{}
	for i, tc := range []struct {
		name     string
		due      *time.Time
		breached bool
	}{
		{name: "overdue", due: ptrTime(now.Add(-time.Minute)), breached: true},
		{name: "soon", due: ptrTime(now.Add(20 * time.Minute))},
		{name: "later", due: ptrTime(now.Add(3 * time.Hour))},
		{name: "finished"},
	} {
		conversation := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: i + 1, Subject: tc.name, Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "widget"}
		if err := db.Create(&conversation).Error; err != nil {
			t.Fatal(err)
		}
		record := model.SupportConversationSLARecord{ConversationID: conversation.ID, WorkspaceID: workspaceID, NextDueAt: tc.due, Breached: tc.breached}
		if err := db.Create(&record).Error; err != nil {
			t.Fatal(err)
		}
		ids[conversation.ID] = tc.name
	}
	conversations := repository.NewSupportConversationRepository(db)
	for filter, want := range map[string]string{"breached": "overdue", "due_soon": "soon", "active": "later,overdue,soon"} {
		list, _, err := conversations.List(context.Background(), repository.ConversationRepositoryListParams{
			ConversationListParams: repository.ConversationListParams{WorkspaceID: workspaceID, SLA: filter}, Role: model.RoleOwner})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, conversation := range list {
			names = append(names, ids[conversation.ID])
		}
		slices.Sort(names)
		if got := strings.Join(names, ","); got != want {
			t.Fatalf("sla=%s: %s, want %s", filter, got, want)
		}
	}
}
