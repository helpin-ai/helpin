//go:build integration

package service

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func seedSupportOpsWorkspace(t *testing.T, db *gorm.DB) (workspaceID, userID string) {
	t.Helper()
	userID, workspaceID = uuid.NewString(), uuid.NewString()
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name) VALUES (?, ?, 'x', 'Ada Agent')`, userID, userID+"@example.com")
	mustExec(t, db, `INSERT INTO workspaces (id, name, slug, workspace_key, owner_id) VALUES (?, 'Ops', ?, ?, ?)`,
		workspaceID, "ops-"+workspaceID[:8], strings.ToUpper(workspaceID[:5]), userID)
	mustExec(t, db, `INSERT INTO support_inbox_state_rollouts (workspace_id, mode) VALUES (?, 'v2')`, workspaceID)
	return workspaceID, userID
}

func sharedInboxCount(t *testing.T, db *gorm.DB, workspaceID string) int {
	t.Helper()
	var total int
	if err := db.Raw(`SELECT COALESCE(SUM(total_count), 0) FROM support_inbox_counter_buckets
		WHERE workspace_id = ? AND audience_type = 'shared' AND bucket_id = 'inbox'`, workspaceID).Scan(&total).Error; err != nil {
		t.Fatal(err)
	}
	return total
}

func TestSupportSnoozePostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	conversations := repository.NewSupportConversationRepository(db)
	messages := repository.NewSupportMessageRepository(db)
	svc := NewSupportInboxService(conversations, repository.NewSupportMailboxRepository(db), messages, nil, nil,
		repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, repository.NewUserRepository(db), nil, nil, nil)

	flow := model.SupportConversationFlowStateWaitingForHuman
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Snooze me", Status: model.SupportConversationStatusOpen, FlowState: &flow, Channel: "widget"}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	if got := sharedInboxCount(t, db, workspaceID); got != 1 {
		t.Fatalf("inbox count before snooze = %d", got)
	}
	if _, err := svc.SnoozeConversation(ctx, workspaceID, conversation.ID, time.Now().Add(-time.Hour), userID); err == nil {
		t.Fatal("snooze in the past accepted")
	}
	until := time.Now().Add(2 * time.Hour)
	if _, err := svc.SnoozeConversation(ctx, workspaceID, conversation.ID, until, userID); err != nil {
		t.Fatal(err)
	}
	if got := sharedInboxCount(t, db, workspaceID); got != 0 {
		t.Fatalf("snoozed conversation still counted in the inbox: %d", got)
	}
	list := func(filter string) int {
		items, _, err := conversations.List(ctx, repository.ConversationRepositoryListParams{ConversationListParams: repository.ConversationListParams{WorkspaceID: workspaceID, Filter: filter}, Role: model.RoleOwner})
		if err != nil {
			t.Fatal(err)
		}
		return len(items)
	}
	if list("inbox") != 0 || list("") != 0 || list("snoozed") != 1 {
		t.Fatalf("lists: inbox=%d all=%d snoozed=%d", list("inbox"), list(""), list("snoozed"))
	}
	var event model.SupportMessage
	if err := db.Where("conversation_id = ? AND system_event_type = ?", conversation.ID, model.SystemEventSnoozed).Take(&event).Error; err != nil || !strings.HasPrefix(event.Content, "Ada snoozed") {
		t.Fatalf("snooze timeline event: %+v %v", event, err)
	}

	// A customer reply wakes it on any write path.
	if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "Any update?"}); err != nil {
		t.Fatal(err)
	}
	var snoozed sql.NullTime
	if err := db.Raw(`SELECT snoozed_until FROM support_conversations WHERE id = ?`, conversation.ID).Row().Scan(&snoozed); err != nil || snoozed.Valid {
		t.Fatalf("customer reply did not wake the conversation: %v %v", snoozed, err)
	}
	if got := sharedInboxCount(t, db, workspaceID); got != 1 {
		t.Fatalf("woken conversation not counted: %d", got)
	}

	// The waker ends due snoozes once, even when run twice.
	if _, err := svc.SnoozeConversation(ctx, workspaceID, conversation.ID, time.Now().Add(time.Hour), userID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE support_conversations SET snoozed_until = now() - interval '1 minute' WHERE id = ?`, conversation.ID)
	first, err := svc.WakeDueSnoozes(ctx, time.Now(), 50)
	if err != nil || first != 1 {
		t.Fatalf("wake: %d %v", first, err)
	}
	second, err := svc.WakeDueSnoozes(ctx, time.Now(), 50)
	if err != nil || second != 0 {
		t.Fatalf("second wake: %d %v", second, err)
	}
	var unsnoozeEvents int64
	db.Model(&model.SupportMessage{}).Where("conversation_id = ? AND system_event_type = ?", conversation.ID, model.SystemEventUnsnoozed).Count(&unsnoozeEvents)
	if unsnoozeEvents != 1 {
		t.Fatalf("unsnooze events = %d", unsnoozeEvents)
	}

	// Resolving clears a snooze.
	if _, err := svc.SnoozeConversation(ctx, workspaceID, conversation.ID, time.Now().Add(time.Hour), userID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateConversationStatus(ctx, workspaceID, conversation.ID, model.SupportConversationStatusResolved, userID); err != nil {
		t.Fatal(err)
	}
	if err := db.Raw(`SELECT snoozed_until FROM support_conversations WHERE id = ?`, conversation.ID).Row().Scan(&snoozed); err != nil || snoozed.Valid {
		t.Fatalf("resolve kept the snooze: %v %v", snoozed, err)
	}
	if _, err := svc.SnoozeConversation(ctx, workspaceID, conversation.ID, time.Now().Add(time.Hour), userID); err == nil {
		t.Fatal("snoozing a resolved conversation was accepted")
	}

	// Priority changes are recorded.
	if _, err := svc.UpdateConversationPriority(ctx, workspaceID, conversation.ID, "urgent", userID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateConversationPriority(ctx, workspaceID, conversation.ID, "critical", userID); err == nil {
		t.Fatal("unknown priority accepted")
	}
	var priority string
	db.Raw(`SELECT priority FROM support_conversations WHERE id = ?`, conversation.ID).Scan(&priority)
	if priority != "urgent" {
		t.Fatalf("priority = %s", priority)
	}
}

func TestSupportSLAQueuePostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	mustExec(t, db, `DELETE FROM support_sla_work`) // isolate from other tests in this transaction
	conversations := repository.NewSupportConversationRepository(db)
	messages := repository.NewSupportMessageRepository(db)
	inbox := NewSupportInboxService(conversations, repository.NewSupportMailboxRepository(db), messages, nil, nil,
		repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, repository.NewUserRepository(db), nil, nil, nil)
	sla := NewSupportSLAService(repository.NewSupportSLARepository(db), inbox)
	inbox.SetSLAService(sla)
	start := time.Now().UTC()
	sla.now = func() time.Time { return start }

	if _, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Nope"}); err == nil {
		t.Fatal("policy without targets accepted")
	}
	policy, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{
		Name: "Urgent", Priorities: []string{"urgent"}, FirstResponseMinutes: intPtr(30), ResolutionMinutes: intPtr(240),
	})
	if err != nil {
		t.Fatal(err)
	}

	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Down", Status: model.SupportConversationStatusOpen, Priority: "urgent", Channel: "email", CreatedAt: start.Add(time.Second)}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "Help", CreatedAt: start.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	var queued int64
	db.Model(&model.SupportSLAWork{}).Where("conversation_id = ?", conversation.ID).Count(&queued)
	if queued != 1 {
		t.Fatalf("triggers did not queue the conversation: %d", queued)
	}
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	record := loadSLARecord(t, db, conversation.ID)
	if record.FirstResponseState != model.SupportSLAStatePending || record.ResolutionState != model.SupportSLAStatePending || record.WakeAt == nil ||
		*record.PolicyID != policy.ID {
		t.Fatalf("pending record: %+v", record)
	}
	detail, err := inbox.GetConversation(ctx, workspaceID, conversation.ID)
	if err != nil || detail.SLA == nil || len(detail.SLA.Metrics) != 2 {
		t.Fatalf("detail SLA: %+v %v", detail, err)
	}

	// Forty minutes later the first-response target is missed, once.
	sla.now = func() time.Time { return start.Add(40 * time.Minute) }
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	record = loadSLARecord(t, db, conversation.ID)
	if record.FirstResponseState != model.SupportSLAStateBreached || !record.Breached {
		t.Fatalf("breached record: %+v", record)
	}
	var breachEvents int64
	db.Model(&model.SupportMessage{}).Where("conversation_id = ? AND system_event_type = ?", conversation.ID, model.SystemEventSLABreached).Count(&breachEvents)
	if breachEvents != 1 {
		t.Fatalf("breach events = %d", breachEvents)
	}
	breachedOnly, _, err := conversations.List(ctx, repository.ConversationRepositoryListParams{ConversationListParams: repository.ConversationListParams{WorkspaceID: workspaceID, SLA: "breached", Sort: repository.SupportConversationSortSLA}, Role: model.RoleOwner})
	if err != nil || len(breachedOnly) != 1 {
		t.Fatalf("breached filter: %d %v", len(breachedOnly), err)
	}

	// A teammate reply is queued by the trigger and closes the open breach.
	userCopy := userID
	if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: "user", SenderUserID: &userCopy, MessageType: "reply", Content: "On it", CreatedAt: start.Add(41 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	sla.now = func() time.Time { return start.Add(42 * time.Minute) }
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	record = loadSLARecord(t, db, conversation.ID)
	if record.FirstResponseState != model.SupportSLAStateBreached || record.FirstResponseAchievedAt == nil || record.Breached {
		t.Fatalf("after late reply: %+v", record)
	}

	// Lowering the priority moves the conversation out of the policy: open
	// targets end, the missed first response stays recorded.
	mustExec(t, db, `UPDATE support_conversations SET priority = 'low' WHERE id = ?`, conversation.ID)
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	record = loadSLARecord(t, db, conversation.ID)
	if record.DetachedAt == nil || record.FirstResponseState != model.SupportSLAStateBreached ||
		record.ResolutionState != model.SupportSLAStateCancelled || record.NextDueAt != nil || record.WakeAt != nil {
		t.Fatalf("record after the policy stopped matching: %+v", record)
	}
	var queuedAfter int64
	db.Model(&model.SupportSLAWork{}).Where("conversation_id = ?", conversation.ID).Count(&queuedAfter)
	if queuedAfter != 0 {
		t.Fatalf("completed work left in the queue: %d", queuedAfter)
	}
	var remaining int64

	// Conversations created before the policy are left alone.
	old := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 2, Subject: "Old", Status: model.SupportConversationStatusOpen, Priority: "urgent", Channel: "email", CreatedAt: start.Add(-time.Hour)}
	if err := db.Create(old).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	db.Model(&model.SupportConversationSLARecord{}).Where("conversation_id = ?", old.ID).Count(&remaining)
	if remaining != 0 {
		t.Fatal("policy back-dated onto an older conversation")
	}
}

func loadSLARecord(t *testing.T, db *gorm.DB, conversationID string) model.SupportConversationSLARecord {
	t.Helper()
	var record model.SupportConversationSLARecord
	if err := db.Where("conversation_id = ?", conversationID).Take(&record).Error; err != nil {
		t.Fatal(err)
	}
	return record
}

func TestSupportSLAAIHandoffPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	mustExec(t, db, `DELETE FROM support_sla_work`)
	messages := repository.NewSupportMessageRepository(db)
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db), messages, nil, nil,
		repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil, repository.NewUserRepository(db), nil, nil, nil)
	sla := NewSupportSLAService(repository.NewSupportSLARepository(db), inbox)
	inbox.SetSLAService(sla)
	start := time.Now().UTC()
	sla.now = func() time.Time { return start }
	if _, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "All", FirstResponseMinutes: intPtr(5)}); err != nil {
		t.Fatal(err)
	}
	flow, pending := model.SupportConversationFlowStateAIHandling, "pending"
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Bot", Status: model.SupportConversationStatusOpen,
		Priority: "medium", Channel: "widget", FlowState: &flow, AIState: &pending, CreatedAt: start.Add(time.Second)}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct {
		sender string
		at     time.Duration
	}{{"customer", time.Second}, {"ai", time.Minute}} {
		if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: m.sender, MessageType: "reply", Content: "x", CreatedAt: start.Add(m.at)}); err != nil {
			t.Fatal(err)
		}
	}
	// Long after the target, the AI still owns the conversation: no breach.
	sla.now = func() time.Time { return start.Add(time.Hour) }
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	record := loadSLARecord(t, db, conversation.ID)
	if !record.AIInvolved || record.FirstResponseState != model.SupportSLAStatePaused || record.Breached {
		t.Fatalf("SLA ran while the AI answered: %+v", record)
	}
	// The escalation is queued by the trigger and starts the clock.
	escalatedAt := start.Add(time.Hour)
	mustExec(t, db, `UPDATE support_conversations SET flow_state = 'waiting_for_human', ai_state = 'escalated', human_takeover = true, ai_escalated_at = ? WHERE id = ?`, escalatedAt, conversation.ID)
	var queued int64
	db.Model(&model.SupportSLAWork{}).Where("conversation_id = ?", conversation.ID).Count(&queued)
	if queued != 1 {
		t.Fatal("handoff did not queue an SLA evaluation")
	}
	sla.now = func() time.Time { return escalatedAt.Add(time.Minute) }
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	record = loadSLARecord(t, db, conversation.ID)
	if record.FirstResponseState != model.SupportSLAStatePending || record.HumanSince == nil ||
		!record.FirstResponseDueAt.Equal(escalatedAt.Add(5*time.Minute).Truncate(time.Microsecond)) {
		t.Fatalf("after handoff: %+v", record)
	}

	// Resolving outside the inbox service still ends a snooze.
	mustExec(t, db, `UPDATE support_conversations SET snoozed_until = ? WHERE id = ?`, start.Add(48*time.Hour), conversation.ID)
	mustExec(t, db, `UPDATE support_conversations SET status = 'resolved', resolved_at = now() WHERE id = ?`, conversation.ID)
	var snoozed sql.NullTime
	if err := db.Raw(`SELECT snoozed_until FROM support_conversations WHERE id = ?`, conversation.ID).Row().Scan(&snoozed); err != nil || snoozed.Valid {
		t.Fatalf("resolved conversation kept its snooze: %v %v", snoozed, err)
	}
}

func TestSupportSLATiersPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	mustExec(t, db, `DELETE FROM support_sla_work`)
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil,
		repository.NewUserRepository(db), nil, nil, nil)
	sla := NewSupportSLAService(repository.NewSupportSLARepository(db), inbox)
	inbox.SetSLAService(sla)
	start := time.Now().UTC()
	sla.now = func() time.Time { return start }

	gold, err := sla.CreateTier(ctx, workspaceID, model.SupportSLATierRequest{Name: "Gold"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sla.CreateTier(ctx, workspaceID, model.SupportSLATierRequest{Name: "GOLD"}); err == nil {
		t.Fatal("duplicate tier name accepted")
	}
	company := model.CRMCompany{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: "C-1", Name: "Acme"}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	policy, err := sla.CreatePolicy(ctx, workspaceID, userID, model.SupportSLAPolicyRequest{Name: "Gold", FirstResponseMinutes: intPtr(30),
		CompanyTierIDs: []string{gold.ID, model.SupportSLANoTier}, CompanyIDs: []string{company.ID}})
	if err != nil || len(policy.Companies) != 1 {
		t.Fatalf("policy %+v %v", policy, err)
	}

	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, DisplayID: 1, Subject: "Down",
		Status: model.SupportConversationStatusOpen, Priority: "medium", Channel: "email", CreatedAt: start.Add(time.Second)}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `DELETE FROM support_sla_work`)
	// Linking the conversation to a company queues it.
	mustExec(t, db, `UPDATE support_conversations SET crm_company_id = ? WHERE id = ?`, company.ID, conversation.ID)
	var queued int64
	db.Model(&model.SupportSLAWork{}).Where("conversation_id = ?", conversation.ID).Count(&queued)
	if queued != 1 {
		t.Fatalf("company change did not queue the conversation: %d", queued)
	}
	var changedAt sql.NullTime
	if err := db.Raw(`SELECT crm_company_changed_at FROM support_conversations WHERE id = ?`, conversation.ID).Scan(&changedAt).Error; err != nil || !changedAt.Valid {
		t.Fatalf("company change time not recorded: %+v %v", changedAt, err)
	}
	mustExec(t, db, `DELETE FROM support_sla_work`)

	if updated, err := sla.SetCompanyTier(ctx, workspaceID, model.SetSupportSLATierRequest{CompanyIDs: []string{company.ID}, TierID: &gold.ID}); err != nil || updated != 1 {
		t.Fatalf("set tier: %d %v", updated, err)
	}
	db.Model(&model.SupportSLAWork{}).Where("conversation_id = ?", conversation.ID).Count(&queued)
	if queued != 1 {
		t.Fatalf("tier change did not queue the conversation: %d", queued)
	}
	view, err := sla.CompanySLA(ctx, workspaceID, company.ID)
	if err != nil || view.TierName == nil || *view.TierName != "Gold" || len(view.Policies) != 1 {
		t.Fatalf("company SLA %+v %v", view, err)
	}

	if err := sla.DeleteTier(ctx, workspaceID, gold.ID); err != nil {
		t.Fatal(err)
	}
	policies, err := sla.ListPolicies(ctx, workspaceID)
	if err != nil || strings.Join(policies[0].CompanyTierIDs, ",") != model.SupportSLANoTier {
		t.Fatalf("tier left in policy: %+v %v", policies, err)
	}
	if _, err := sla.CustomizeCompanySLA(ctx, workspaceID, userID, company.ID); err != nil {
		t.Fatal(err)
	}
	if err := sla.RemoveCompanySLA(ctx, workspaceID, company.ID); err != nil {
		t.Fatal(err)
	}
}
