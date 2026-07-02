package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type scriptedSupportTriageLLM struct {
	response string
	err      error
	calls    int
	lastReq  llm.ChatRequest
}

func (f *scriptedSupportTriageLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ChatResponse{Content: f.response}, nil
}

type supportTriageTestFixture struct {
	db               *gorm.DB
	ctx              context.Context
	workspaceID      string
	actorID          string
	conversationRepo *repository.SupportConversationRepository
	messageRepo      *repository.SupportMessageRepository
	mailboxRepo      *repository.SupportMailboxRepository
	triageRepo       *repository.SupportConversationTriageRepository
	eventRepo        *repository.SupportConversationTriageEventRepository
	ruleRepo         *repository.SupportTriageRuleRepository
	supportSvc       *SupportInboxService
	triageSvc        *SupportInboxTriageService
}

func newSupportTriageTestFixture(t *testing.T, llmProvider llm.Provider, mutateSettings func(*model.SupportInboxSettings)) *supportTriageTestFixture {
	t.Helper()

	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-triage"
	actorID := "user-support-owner"

	seedUser(t, db, actorID, "owner@example.com", "Support Owner", "hash")
	seedWorkspace(t, db, workspaceID, "Support Workspace", "support-workspace", actorID)
	seedWorkspaceMember(t, db, "wm-support-owner", workspaceID, actorID, "owner@example.com", "Support Owner", model.RoleAdmin)
	seedSupportInstallationSettings(t, db, workspaceID, func(settings *model.SupportInboxSettings) {
		settings.TriageEnabled = true
		settings.TriageWidgetEnabled = true
		settings.TriageEmailEnabled = true
		settings.TriageInternalEnabled = false
		if mutateSettings != nil {
			mutateSettings(settings)
		}
	})

	conversationRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	mailboxRepo := repository.NewSupportMailboxRepository(db)
	installRepo := repository.NewSupportInboxInstallationRepository(db)
	triageRepo := repository.NewSupportConversationTriageRepository(db)
	eventRepo := repository.NewSupportConversationTriageEventRepository(db)
	ruleRepo := repository.NewSupportTriageRuleRepository(db)
	userRepo := repository.NewUserRepository(db)

	supportSvc := NewSupportInboxService(
		conversationRepo,
		mailboxRepo,
		messageRepo,
		nil,
		nil,
		installRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		userRepo,
		nil,
		nil,
		nil,
	)
	triageSvc := NewSupportInboxTriageService(
		supportSvc,
		triageRepo,
		eventRepo,
		ruleRepo,
		installRepo,
		mailboxRepo,
		conversationRepo,
		messageRepo,
		llmProvider,
	)
	supportSvc.SetTriageService(triageSvc)

	return &supportTriageTestFixture{
		db:               db,
		ctx:              ctx,
		workspaceID:      workspaceID,
		actorID:          actorID,
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		mailboxRepo:      mailboxRepo,
		triageRepo:       triageRepo,
		eventRepo:        eventRepo,
		ruleRepo:         ruleRepo,
		supportSvc:       supportSvc,
		triageSvc:        triageSvc,
	}
}

func (f *supportTriageTestFixture) createMailbox(t *testing.T, name, handle string, triageEligible bool) *model.SupportMailbox {
	t.Helper()

	mailbox := &model.SupportMailbox{
		WorkspaceID:    f.workspaceID,
		Name:           name,
		Handle:         handle,
		Icon:           "inbox",
		TriageEligible: triageEligible,
		AssignmentMode: "manual",
		Active:         true,
		CreatedByID:    f.actorID,
	}
	if err := f.mailboxRepo.Create(f.ctx, mailbox); err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	created, err := f.mailboxRepo.GetByID(f.ctx, f.workspaceID, mailbox.ID)
	if err != nil {
		t.Fatalf("get mailbox: %v", err)
	}
	return created
}

func (f *supportTriageTestFixture) createMailboxWithRoutingPrompt(t *testing.T, name, handle, routingPrompt string, triageEligible bool) *model.SupportMailbox {
	t.Helper()

	mailbox := f.createMailbox(t, name, handle, triageEligible)
	mailbox.RoutingPrompt = strPtr(routingPrompt)
	if err := f.mailboxRepo.Update(f.ctx, mailbox); err != nil {
		t.Fatalf("update mailbox routing prompt: %v", err)
	}
	updated, err := f.mailboxRepo.GetByID(f.ctx, f.workspaceID, mailbox.ID)
	if err != nil {
		t.Fatalf("get updated mailbox: %v", err)
	}
	return updated
}

func (f *supportTriageTestFixture) createConversation(t *testing.T, subject, customerEmail string, mailboxID *string) *model.SupportConversation {
	t.Helper()

	conversation := &model.SupportConversation{
		WorkspaceID:   f.workspaceID,
		MailboxID:     mailboxID,
		Subject:       subject,
		Status:        "open",
		Priority:      "medium",
		Channel:       "widget",
		Source:        "widget",
		CustomerEmail: strPtr(customerEmail),
		CustomerName:  strPtr("Test Customer"),
	}
	if err := f.conversationRepo.Create(f.ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return conversation
}

func (f *supportTriageTestFixture) createCustomerReply(t *testing.T, conversationID, content string) *model.SupportMessage {
	t.Helper()

	message := &model.SupportMessage{
		WorkspaceID:       f.workspaceID,
		ConversationID:    conversationID,
		SenderType:        "customer",
		SenderDisplayName: strPtr("Test Customer"),
		Content:           content,
		MessageType:       "reply",
	}
	if err := f.messageRepo.Create(f.ctx, message); err != nil {
		t.Fatalf("create customer reply: %v", err)
	}
	return message
}

func loadSystemMessages(t *testing.T, repo *repository.SupportMessageRepository, workspaceID, conversationID string) []model.SupportMessage {
	t.Helper()

	messages, err := repo.ListByConversation(context.Background(), workspaceID, conversationID, true)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	result := make([]model.SupportMessage, 0)
	for _, message := range messages {
		if strings.TrimSpace(message.MessageType) == "system" {
			result = append(result, message)
		}
	}
	return result
}

func countTriageEvents(t *testing.T, db *gorm.DB, conversationID string, eventType string) int64 {
	t.Helper()

	var count int64
	if err := db.Model(&model.SupportConversationTriageEvent{}).
		Where("conversation_id = ? AND event_type = ?", conversationID, eventType).
		Count(&count).Error; err != nil {
		t.Fatalf("count triage events: %v", err)
	}
	return count
}

func TestSupportInboxTriageEvaluateAndRoute_RuleSuggestion(t *testing.T) {
	fakeLLM := &scriptedSupportTriageLLM{
		response: `{"intent":"marketing_guest_post","target_mailbox_handle":"marketing","confidence":0.95,"reason":"guest post outreach"}`,
	}
	fixture := newSupportTriageTestFixture(t, fakeLLM, func(settings *model.SupportInboxSettings) {
		settings.TriageAutoMoveEnabled = false
	})
	marketing := fixture.createMailbox(t, "Marketing", "marketing", true)

	if _, err := fixture.triageSvc.CreateRule(fixture.ctx, fixture.workspaceID, fixture.actorID, model.CreateSupportTriageRuleRequest{
		Name:            "Guest post",
		Priority:        1,
		Channels:        []string{"widget"},
		Conditions:      model.SupportTriageRuleConditions{PhraseContains: []string{"write for us"}},
		TargetMailboxID: marketing.ID,
	}); err != nil {
		t.Fatalf("create triage rule: %v", err)
	}

	conversation := fixture.createConversation(t, "Quick question", "guest@publisher.com", nil)
	message := fixture.createCustomerReply(t, conversation.ID, "Hi, I wanted to write for us about your product.")

	triage, err := fixture.triageSvc.EvaluateAndRoute(fixture.ctx, fixture.workspaceID, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("EvaluateAndRoute: %v", err)
	}
	if triage == nil {
		t.Fatal("expected triage result")
	}
	if triage.Status != model.SupportConversationTriageStatusSuggested {
		t.Fatalf("status = %q, want %q", triage.Status, model.SupportConversationTriageStatusSuggested)
	}
	if triage.ClassifierSource != model.SupportConversationTriageSourceRule {
		t.Fatalf("source = %q, want %q", triage.ClassifierSource, model.SupportConversationTriageSourceRule)
	}
	if derefString(triage.Reason) != "Routing rule matched." {
		t.Fatalf("reason = %q, want generic rule reason", derefString(triage.Reason))
	}
	if derefString(triage.SuggestedMailboxID) != marketing.ID {
		t.Fatalf("suggested_mailbox_id = %q, want %q", derefString(triage.SuggestedMailboxID), marketing.ID)
	}
	if fakeLLM.calls != 0 {
		t.Fatalf("llm calls = %d, want 0", fakeLLM.calls)
	}

	updatedConversation, err := fixture.conversationRepo.GetByID(fixture.ctx, fixture.workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if updatedConversation.MailboxID != nil {
		t.Fatalf("mailbox_id = %q, want shared", derefString(updatedConversation.MailboxID))
	}
	if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventEvaluated); got != 1 {
		t.Fatalf("evaluated events = %d, want 1", got)
	}
}

func TestSupportInboxTriageEvaluateAndRoute_AISuggestion(t *testing.T) {
	fakeLLM := &scriptedSupportTriageLLM{
		response: `{"intent":"sales_pricing","target_mailbox_handle":"sales","confidence":0.82,"reason":"pricing request"}`,
	}
	fixture := newSupportTriageTestFixture(t, fakeLLM, nil)
	sales := fixture.createMailbox(t, "Sales", "sales", true)

	conversation := fixture.createConversation(t, "Need pricing", "buyer@enterprise.com", nil)
	message := fixture.createCustomerReply(t, conversation.ID, "Can someone walk me through enterprise pricing and procurement?")

	triage, err := fixture.triageSvc.EvaluateAndRoute(fixture.ctx, fixture.workspaceID, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("EvaluateAndRoute: %v", err)
	}
	if triage == nil {
		t.Fatal("expected triage result")
	}
	if triage.ClassifierSource != model.SupportConversationTriageSourceAI {
		t.Fatalf("source = %q, want %q", triage.ClassifierSource, model.SupportConversationTriageSourceAI)
	}
	if derefString(triage.Reason) != "pricing request" {
		t.Fatalf("reason = %q, want AI reason", derefString(triage.Reason))
	}
	if derefString(triage.SuggestedMailboxID) != sales.ID {
		t.Fatalf("suggested_mailbox_id = %q, want %q", derefString(triage.SuggestedMailboxID), sales.ID)
	}
	if fakeLLM.calls != 1 {
		t.Fatalf("llm calls = %d, want 1", fakeLLM.calls)
	}
	if !strings.Contains(fakeLLM.lastReq.Messages[0].Content, "Shared Inbox") {
		t.Fatalf("triage prompt did not include shared inbox option")
	}
}

func TestSupportInboxTriageEvaluateAndRoute_AISharedDoesNotUseLexicalFallback(t *testing.T) {
	fakeLLM := &scriptedSupportTriageLLM{
		response: `{"intent":"unclear","target_mailbox_handle":"shared","confidence":0.22,"reason":"No specialized inbox is a clear fit."}`,
	}
	fixture := newSupportTriageTestFixture(t, fakeLLM, nil)
	fixture.createMailboxWithRoutingPrompt(t, "Billing", "billing", "Messages from customers about invoices and billing.", true)

	conversation := fixture.createConversation(t, "Message from customer", "buyer@example.com", nil)
	message := fixture.createCustomerReply(t, conversation.ID, "This is a message from the customer. Can someone take a look?")

	triage, err := fixture.triageSvc.EvaluateAndRoute(fixture.ctx, fixture.workspaceID, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("EvaluateAndRoute: %v", err)
	}
	if triage != nil {
		t.Fatalf("triage = %#v, want nil when AI abstains", triage)
	}
	if fakeLLM.calls != 1 {
		t.Fatalf("llm calls = %d, want 1", fakeLLM.calls)
	}

	updatedConversation, err := fixture.conversationRepo.GetByID(fixture.ctx, fixture.workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if updatedConversation.MailboxID != nil {
		t.Fatalf("mailbox_id = %q, want shared", derefString(updatedConversation.MailboxID))
	}
	if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventEvaluated); got != 0 {
		t.Fatalf("evaluated events = %d, want 0", got)
	}
}

func TestSupportInboxTriageEvaluateAndRoute_AutoMovesHighConfidenceRule(t *testing.T) {
	fixture := newSupportTriageTestFixture(t, nil, func(settings *model.SupportInboxSettings) {
		settings.TriageAutoMoveEnabled = true
		settings.TriageConfidenceThreshold = 0.9
	})
	billing := fixture.createMailbox(t, "Billing", "billing", true)

	if _, err := fixture.triageSvc.CreateRule(fixture.ctx, fixture.workspaceID, fixture.actorID, model.CreateSupportTriageRuleRequest{
		Name:            "Refunds",
		Priority:        1,
		Channels:        []string{"widget"},
		Conditions:      model.SupportTriageRuleConditions{PhraseContains: []string{"refund"}},
		TargetMailboxID: billing.ID,
	}); err != nil {
		t.Fatalf("create triage rule: %v", err)
	}

	conversation := fixture.createConversation(t, "Refund request", "buyer@example.com", nil)
	message := fixture.createCustomerReply(t, conversation.ID, "I need a refund for last month.")

	triage, err := fixture.triageSvc.EvaluateAndRoute(fixture.ctx, fixture.workspaceID, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("EvaluateAndRoute: %v", err)
	}
	if triage == nil {
		t.Fatal("expected triage result")
	}
	if triage.Status != model.SupportConversationTriageStatusAutoMoved {
		t.Fatalf("status = %q, want %q", triage.Status, model.SupportConversationTriageStatusAutoMoved)
	}
	if !triage.AutoMoved {
		t.Fatal("expected auto_moved = true")
	}

	updatedConversation, err := fixture.conversationRepo.GetByID(fixture.ctx, fixture.workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if derefString(updatedConversation.MailboxID) != billing.ID {
		t.Fatalf("mailbox_id = %q, want %q", derefString(updatedConversation.MailboxID), billing.ID)
	}
	if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventAutoMoved); got != 1 {
		t.Fatalf("auto_moved events = %d, want 1", got)
	}

	systemMessages := loadSystemMessages(t, fixture.messageRepo, fixture.workspaceID, conversation.ID)
	if len(systemMessages) == 0 {
		t.Fatal("expected auto-move system message")
	}
	if !systemMessages[len(systemMessages)-1].IsInternal {
		t.Fatal("expected auto-move system message to be internal")
	}
	if !strings.Contains(systemMessages[len(systemMessages)-1].Content, "Routing rule moved to inbox 'Billing'.") {
		t.Fatalf("unexpected system message %q", systemMessages[len(systemMessages)-1].Content)
	}
}

func TestSupportInboxTriageShouldAutoMoveRequiresTrustedSource(t *testing.T) {
	targetMailboxID := "mailbox-target"
	confidence := 1.0
	settings := model.DefaultSupportInboxSettings()
	settings.TriageAutoMoveEnabled = true
	settings.TriageConfidenceThreshold = 0.8
	conversation := &model.SupportConversation{
		Channel: "widget",
		Source:  "widget",
	}
	messages := []model.SupportMessage{
		{
			SenderType:  "customer",
			MessageType: "reply",
		},
	}

	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{name: "rule source", source: model.SupportConversationTriageSourceRule, want: true},
		{name: "ai source", source: model.SupportConversationTriageSourceAI, want: true},
		{name: "lexical source", source: "lexical", want: false},
		{name: "empty source", source: "", want: false},
	}

	svc := &SupportInboxTriageService{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triage := &model.SupportConversationTriage{
				ClassifierSource:   tt.source,
				Confidence:         &confidence,
				SuggestedMailboxID: &targetMailboxID,
			}
			if got := svc.shouldAutoMove(settings, conversation, messages, triage); got != tt.want {
				t.Fatalf("shouldAutoMove() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportInboxTriageEvaluateAndRoute_AutoMovesHighConfidenceSenderEmailContainsRule(t *testing.T) {
	fixture := newSupportTriageTestFixture(t, nil, func(settings *model.SupportInboxSettings) {
		settings.TriageAutoMoveEnabled = true
		settings.TriageConfidenceThreshold = 0.9
	})
	billing := fixture.createMailbox(t, "Billing", "billing", true)

	if _, err := fixture.triageSvc.CreateRule(fixture.ctx, fixture.workspaceID, fixture.actorID, model.CreateSupportTriageRuleRequest{
		Name:            "VIP sender",
		Priority:        1,
		Channels:        []string{"widget"},
		Conditions:      model.SupportTriageRuleConditions{SenderEmailContains: []string{"vip@"}},
		TargetMailboxID: billing.ID,
	}); err != nil {
		t.Fatalf("create triage rule: %v", err)
	}

	conversation := fixture.createConversation(t, "Question", "vip@acme.com", nil)
	message := fixture.createCustomerReply(t, conversation.ID, "Can you help me?")

	triage, err := fixture.triageSvc.EvaluateAndRoute(fixture.ctx, fixture.workspaceID, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("EvaluateAndRoute: %v", err)
	}
	if triage == nil {
		t.Fatal("expected triage result")
	}
	if triage.Status != model.SupportConversationTriageStatusAutoMoved {
		t.Fatalf("status = %q, want %q", triage.Status, model.SupportConversationTriageStatusAutoMoved)
	}

	updatedConversation, err := fixture.conversationRepo.GetByID(fixture.ctx, fixture.workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if derefString(updatedConversation.MailboxID) != billing.ID {
		t.Fatalf("mailbox_id = %q, want %q", derefString(updatedConversation.MailboxID), billing.ID)
	}
}

func TestSupportInboxTriageEvaluateAndRoute_AutoMovesHighConfidenceAnyConditionRule(t *testing.T) {
	fixture := newSupportTriageTestFixture(t, nil, func(settings *model.SupportInboxSettings) {
		settings.TriageAutoMoveEnabled = true
		settings.TriageConfidenceThreshold = 0.9
	})
	billing := fixture.createMailbox(t, "Billing", "billing", true)

	if _, err := fixture.triageSvc.CreateRule(fixture.ctx, fixture.workspaceID, fixture.actorID, model.CreateSupportTriageRuleRequest{
		Name:     "Billing text or sender",
		Priority: 1,
		Channels: []string{"widget"},
		Conditions: model.SupportTriageRuleConditions{
			ConditionLogic:      "any",
			PhraseContains:      []string{"billing"},
			SenderEmailContains: []string{"billing@"},
		},
		TargetMailboxID: billing.ID,
	}); err != nil {
		t.Fatalf("create triage rule: %v", err)
	}

	conversation := fixture.createConversation(t, "Question", "billing@acme.com", nil)
	message := fixture.createCustomerReply(t, conversation.ID, "Can you help me?")

	triage, err := fixture.triageSvc.EvaluateAndRoute(fixture.ctx, fixture.workspaceID, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("EvaluateAndRoute: %v", err)
	}
	if triage == nil {
		t.Fatal("expected triage result")
	}

	updatedConversation, err := fixture.conversationRepo.GetByID(fixture.ctx, fixture.workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if derefString(updatedConversation.MailboxID) != billing.ID {
		t.Fatalf("mailbox_id = %q, want %q", derefString(updatedConversation.MailboxID), billing.ID)
	}
}

func TestSupportInboxTriageEvaluateAndRoute_SkipsHumanOwnedConversation(t *testing.T) {
	fixture := newSupportTriageTestFixture(t, nil, func(settings *model.SupportInboxSettings) {
		settings.TriageAutoMoveEnabled = true
		settings.TriageConfidenceThreshold = 0.9
	})
	billing := fixture.createMailbox(t, "Billing", "billing", true)

	if _, err := fixture.triageSvc.CreateRule(fixture.ctx, fixture.workspaceID, fixture.actorID, model.CreateSupportTriageRuleRequest{
		Name:            "Refunds",
		Priority:        1,
		Channels:        []string{"widget"},
		Conditions:      model.SupportTriageRuleConditions{PhraseContains: []string{"refund"}},
		TargetMailboxID: billing.ID,
	}); err != nil {
		t.Fatalf("create triage rule: %v", err)
	}

	conversation := fixture.createConversation(t, "Refund request", "buyer@example.com", nil)
	conversation.AssignedUserID = &fixture.actorID
	conversation.HumanTakeover = boolPtr(true)
	conversation.FlowState = strPtr(model.SupportConversationFlowStateAssignedToHuman)
	if err := fixture.conversationRepo.Update(fixture.ctx, conversation); err != nil {
		t.Fatalf("mark conversation human-owned: %v", err)
	}

	message := fixture.createCustomerReply(t, conversation.ID, "I need a refund for last month.")
	triage, err := fixture.triageSvc.EvaluateAndRoute(fixture.ctx, fixture.workspaceID, conversation.ID, message.ID)
	if err != nil {
		t.Fatalf("EvaluateAndRoute: %v", err)
	}
	if triage != nil {
		t.Fatalf("triage = %#v, want nil for human-owned conversation", triage)
	}

	updatedConversation, err := fixture.conversationRepo.GetByID(fixture.ctx, fixture.workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if updatedConversation.MailboxID != nil {
		t.Fatalf("mailbox_id = %q, want shared", derefString(updatedConversation.MailboxID))
	}
	if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventAutoMoved); got != 0 {
		t.Fatalf("auto_moved events = %d, want 0", got)
	}
	if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventEvaluated); got != 0 {
		t.Fatalf("evaluated events = %d, want 0", got)
	}
}

func TestSupportInboxTriageDismissConversation(t *testing.T) {
	fixture := newSupportTriageTestFixture(t, nil, nil)
	marketing := fixture.createMailbox(t, "Marketing", "marketing", true)

	conversation := fixture.createConversation(t, "Partnership idea", "partner@agency.com", nil)
	if err := fixture.triageRepo.Upsert(fixture.ctx, &model.SupportConversationTriage{
		WorkspaceID:        fixture.workspaceID,
		ConversationID:     conversation.ID,
		Status:             model.SupportConversationTriageStatusSuggested,
		Intent:             strPtr("marketing_partnership"),
		Confidence:         float64Ptr(0.91),
		Reason:             strPtr("Partnership language detected"),
		ClassifierSource:   model.SupportConversationTriageSourceAI,
		SuggestedMailboxID: &marketing.ID,
	}); err != nil {
		t.Fatalf("seed triage: %v", err)
	}

	triage, err := fixture.triageSvc.DismissConversationTriage(fixture.ctx, fixture.workspaceID, conversation.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("DismissConversationTriage: %v", err)
	}
	if triage.Status != model.SupportConversationTriageStatusDismissed {
		t.Fatalf("status = %q, want %q", triage.Status, model.SupportConversationTriageStatusDismissed)
	}
	if triage.LockedAt == nil {
		t.Fatal("expected locked_at to be set")
	}
	if derefString(triage.FeedbackAction) != model.SupportConversationTriageFeedbackDismissed {
		t.Fatalf("feedback_action = %q, want %q", derefString(triage.FeedbackAction), model.SupportConversationTriageFeedbackDismissed)
	}
	if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventDismissed); got != 1 {
		t.Fatalf("dismissed events = %d, want 1", got)
	}

	systemMessages := loadSystemMessages(t, fixture.messageRepo, fixture.workspaceID, conversation.ID)
	if len(systemMessages) == 0 {
		t.Fatal("expected dismissal system message")
	}
	if systemMessages[len(systemMessages)-1].Content != "Support dismissed the routing suggestion." {
		t.Fatalf("unexpected system message %q", systemMessages[len(systemMessages)-1].Content)
	}
}

func TestSupportInboxTriageManualMoveFeedback(t *testing.T) {
	fixture := newSupportTriageTestFixture(t, nil, nil)
	sales := fixture.createMailbox(t, "Sales", "sales", true)
	billing := fixture.createMailbox(t, "Billing", "billing", true)

	t.Run("accepted suggestion", func(t *testing.T) {
		conversation := fixture.createConversation(t, "Need pricing", "buyer@enterprise.com", nil)
		if err := fixture.triageRepo.Upsert(fixture.ctx, &model.SupportConversationTriage{
			WorkspaceID:        fixture.workspaceID,
			ConversationID:     conversation.ID,
			Status:             model.SupportConversationTriageStatusSuggested,
			Intent:             strPtr("sales_pricing"),
			Confidence:         float64Ptr(0.84),
			Reason:             strPtr("Pricing language detected"),
			ClassifierSource:   model.SupportConversationTriageSourceAI,
			SuggestedMailboxID: &sales.ID,
		}); err != nil {
			t.Fatalf("seed triage: %v", err)
		}

		if _, err := fixture.supportSvc.moveConversationInternal(fixture.ctx, fixture.workspaceID, conversation.ID, &sales.ID, fixture.actorID, supportConversationMoveOptions{
			EnforceMailboxAccess: false,
			UseAccessibleLoad:    false,
			RecordTriageFeedback: true,
		}); err != nil {
			t.Fatalf("moveConversationInternal: %v", err)
		}

		triage, err := fixture.triageRepo.GetByConversation(fixture.ctx, fixture.workspaceID, conversation.ID)
		if err != nil {
			t.Fatalf("load triage: %v", err)
		}
		if triage.LockedAt == nil {
			t.Fatal("expected locked_at to be set")
		}
		if derefString(triage.FeedbackAction) != model.SupportConversationTriageFeedbackAccepted {
			t.Fatalf("feedback_action = %q, want %q", derefString(triage.FeedbackAction), model.SupportConversationTriageFeedbackAccepted)
		}
		if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventAccepted); got != 1 {
			t.Fatalf("accepted events = %d, want 1", got)
		}

		systemMessages := loadSystemMessages(t, fixture.messageRepo, fixture.workspaceID, conversation.ID)
		if len(systemMessages) == 0 {
			t.Fatal("expected manual move system message")
		}
		if !strings.Contains(systemMessages[len(systemMessages)-1].Content, "Support moved to inbox 'Sales'.") {
			t.Fatalf("unexpected system message %q", systemMessages[len(systemMessages)-1].Content)
		}
	})

	t.Run("corrected suggestion", func(t *testing.T) {
		conversation := fixture.createConversation(t, "Need a refund", "buyer@enterprise.com", nil)
		if err := fixture.triageRepo.Upsert(fixture.ctx, &model.SupportConversationTriage{
			WorkspaceID:        fixture.workspaceID,
			ConversationID:     conversation.ID,
			Status:             model.SupportConversationTriageStatusSuggested,
			Intent:             strPtr("sales_pricing"),
			Confidence:         float64Ptr(0.8),
			Reason:             strPtr("Pricing language detected"),
			ClassifierSource:   model.SupportConversationTriageSourceAI,
			SuggestedMailboxID: &sales.ID,
		}); err != nil {
			t.Fatalf("seed triage: %v", err)
		}

		if _, err := fixture.supportSvc.moveConversationInternal(fixture.ctx, fixture.workspaceID, conversation.ID, &billing.ID, fixture.actorID, supportConversationMoveOptions{
			EnforceMailboxAccess: false,
			UseAccessibleLoad:    false,
			RecordTriageFeedback: true,
		}); err != nil {
			t.Fatalf("moveConversationInternal: %v", err)
		}

		triage, err := fixture.triageRepo.GetByConversation(fixture.ctx, fixture.workspaceID, conversation.ID)
		if err != nil {
			t.Fatalf("load triage: %v", err)
		}
		if triage.Status != model.SupportConversationTriageStatusOverridden {
			t.Fatalf("status = %q, want %q", triage.Status, model.SupportConversationTriageStatusOverridden)
		}
		if derefString(triage.FeedbackAction) != model.SupportConversationTriageFeedbackCorrected {
			t.Fatalf("feedback_action = %q, want %q", derefString(triage.FeedbackAction), model.SupportConversationTriageFeedbackCorrected)
		}
		if got := countTriageEvents(t, fixture.db, conversation.ID, supportTriageEventCorrected); got != 1 {
			t.Fatalf("corrected events = %d, want 1", got)
		}

		systemMessages := loadSystemMessages(t, fixture.messageRepo, fixture.workspaceID, conversation.ID)
		if len(systemMessages) == 0 {
			t.Fatal("expected manual move system message")
		}
		if !strings.Contains(systemMessages[len(systemMessages)-1].Content, "Support moved to inbox 'Billing'.") {
			t.Fatalf("unexpected system message %q", systemMessages[len(systemMessages)-1].Content)
		}
	})
}

func float64Ptr(value float64) *float64 {
	return &value
}
