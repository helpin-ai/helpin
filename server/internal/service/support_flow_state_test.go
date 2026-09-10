package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func TestCreateConversationSetsAssignedToHumanFlowState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-flow-create"
	actorID := "user-flow-owner"
	seedUser(t, db, actorID, "owner@example.com", "Owner User", "hash")
	seedWorkspace(t, db, workspaceID, "Flow Create WS", "flow-create-ws", actorID)

	svc := NewSupportInboxService(
		repository.NewSupportConversationRepository(db),
		repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		nil,
		nil,
		nil,
	)

	conv, err := svc.CreateConversation(ctx, model.CreateConversationRequest{
		WorkspaceID: workspaceID,
		Subject:     "Need manual support",
	}, actorID)
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	if conv.FlowState == nil || *conv.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", conv.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}
}

func TestCreateConversationMessageHumanReplySetsAssignedToHumanFlowState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-flow-reply"
	userID := "user-flow-reply"
	seedUser(t, db, userID, "agent@example.com", "Agent User", "hash")
	seedWorkspace(t, db, workspaceID, "Flow Reply WS", "flow-reply-ws", userID)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	userRepo := repository.NewUserRepository(db)

	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Escalated conversation",
		Status:      "open",
		FlowState:   strPtr(model.SupportConversationFlowStateWaitingForHuman),
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	customerMessage := &model.SupportMessage{
		WorkspaceID: workspaceID, ConversationID: conv.ID, SenderType: "customer",
		MessageType: "reply", Content: "I still need help.",
	}
	if err := messageRepo.Create(ctx, customerMessage); err != nil {
		t.Fatalf("create customer message: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		nil,
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

	displayName := "Agent User"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "I can help with this.", MessageType: "reply"},
		"user",
		&userID,
		nil,
		&displayName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}
	if updated.OpenedByUserID == nil || *updated.OpenedByUserID != userID {
		t.Fatalf("opened_by_user_id = %#v, want %q", updated.OpenedByUserID, userID)
	}
	if updated.HumanTakeover == nil || !*updated.HumanTakeover {
		t.Fatalf("human_takeover = %#v, want true", updated.HumanTakeover)
	}
	if updated.TeamLastSeenAt == nil {
		t.Fatal("team_last_seen_at = nil, want teammate reply to advance the read cursor")
	}
	if updated.UnreadCount != 0 {
		t.Fatalf("unread_count = %d, want 0 after teammate reply", updated.UnreadCount)
	}
}

func TestCreateConversationMessageHumanReplyReopensResolvedConversationForCustomerEmail(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-flow-human-resolved-reply"
	userID := "user-flow-human-resolved-reply"
	seedUser(t, db, userID, "agent-resolved-reply@example.com", "Agent Resolved Reply", "hash")
	seedWorkspace(t, db, workspaceID, "Flow Human Resolved Reply WS", "flow-human-resolved-reply-ws", userID)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	userRepo := repository.NewUserRepository(db)

	resolvedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	conv := &model.SupportConversation{
		WorkspaceID:       workspaceID,
		Subject:           "Resolved conversation with new team reply",
		Status:            model.SupportConversationStatusResolved,
		FlowState:         strPtr(model.SupportConversationFlowStateResolvedByHuman),
		OpenedByUserID:    &userID,
		HumanTakeover:     boolPtr(true),
		ResolvedAt:        &resolvedAt,
		ClosedAt:          &resolvedAt,
		CustomerEmail:     strPtr("customer@example.com"),
		ContactLastSeenAt: nil,
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		nil,
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

	displayName := "Agent Resolved Reply"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "Following up over email.", MessageType: "reply"},
		"user",
		&userID,
		nil,
		&displayName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != model.SupportConversationStatusWaitingOnCustomer {
		t.Fatalf("status = %q, want %q", updated.Status, model.SupportConversationStatusWaitingOnCustomer)
	}
	if updated.ResolvedAt != nil {
		t.Fatalf("resolved_at = %#v, want nil", updated.ResolvedAt)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("closed_at = %#v, want nil", updated.ClosedAt)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}
	if updated.HumanTakeover == nil || !*updated.HumanTakeover {
		t.Fatalf("human_takeover = %#v, want true", updated.HumanTakeover)
	}
}

func TestCreateConversationMessageCustomerReplyPreservesHumanTakeoverFlowState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-human-reopen"
	userID := "user-human-reopen"
	seedUser(t, db, userID, "agent-reopen@example.com", "Agent Reopen", "hash")
	seedWorkspace(t, db, workspaceID, "Human Reopen WS", "human-reopen-ws", userID)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	aiResolved := "resolved"
	conv := &model.SupportConversation{
		WorkspaceID:      workspaceID,
		Subject:          "Handled by a human",
		Status:           model.SupportConversationStatusResolved,
		OpenedByUserID:   &userID,
		AssignedUserID:   &userID,
		AIState:          &aiResolved,
		FlowState:        strPtr(model.SupportConversationFlowStateResolvedByHuman),
		HumanTakeover:    boolPtr(true),
		ResolvedAt:       &time.Time{},
		AIResolvedAt:     &time.Time{},
		AIResolutionType: strPtr("confirmed"),
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		nil,
	)

	customerName := "Customer"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "Following up", MessageType: "reply"},
		"customer",
		nil,
		nil,
		&customerName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != model.SupportConversationStatusOpen {
		t.Fatalf("status = %q, want %q", updated.Status, model.SupportConversationStatusOpen)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}
	if updated.HumanTakeover == nil || !*updated.HumanTakeover {
		t.Fatalf("human_takeover = %#v, want true", updated.HumanTakeover)
	}
}

func TestAssignConversationUserSetsHumanTakeover(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	ensureSupportModuleGrantsTable(t, db)

	workspaceID := "ws-human-assign"
	ownerID := "user-human-assign-owner"
	userID := "user-human-assign"
	seedUser(t, db, ownerID, "owner-assign@example.com", "Owner Assign", "hash")
	seedUser(t, db, userID, "agent-assign@example.com", "Agent Assign", "hash")
	seedWorkspace(t, db, workspaceID, "Human Assign WS", "human-assign-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-human-assign-owner", workspaceID, ownerID, "owner-assign@example.com", "Owner Assign", model.RoleOwner)
	seedWorkspaceMember(t, db, "wm-human-assign", workspaceID, userID, "agent-assign@example.com", "Agent Assign", model.RoleAdmin)

	convRepo := repository.NewSupportConversationRepository(db)
	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Assign to teammate",
		Status:      model.SupportConversationStatusOpen,
		FlowState:   strPtr(model.SupportConversationFlowStateAIHandling),
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		nil,
	).SetWorkspaceRepo(repository.NewWorkspaceRepository(db))

	if err := svc.AssignConversationUser(ctx, workspaceID, conv.ID, &userID, ownerID); err != nil {
		t.Fatalf("AssignConversationUser: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.HumanTakeover == nil || !*updated.HumanTakeover {
		t.Fatalf("human_takeover = %#v, want true", updated.HumanTakeover)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}
	if updated.AssignedUserID == nil || *updated.AssignedUserID != userID {
		t.Fatalf("assigned_user_id = %#v, want %q", updated.AssignedUserID, userID)
	}
}

func TestSupportAIServiceEscalateToHumanSetsAfterHoursQueueFlowState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-flow-escalate"
	seedWorkspace(t, db, workspaceID, "Flow Escalate WS", "flow-escalate-ws", "user-123")

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	handoffRepo := repository.NewAgentHandoffRepository(db)
	instRepo := repository.NewSupportInboxInstallationRepository(db)

	aiPending := "pending"
	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Need a human",
		Status:      "open",
		AIState:     &aiPending,
		FlowState:   strPtr(model.SupportConversationFlowStateAIHandling),
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = true
	settings.BusinessHoursTimezone = "UTC"
	settings.OutsideHoursMessage = "Our team is offline right now."
	settings.BusinessHoursSchedule = map[string]model.BusinessHoursDay{
		"mon": {Start: "09:00", End: "17:00", Enabled: false},
		"tue": {Start: "09:00", End: "17:00", Enabled: false},
		"wed": {Start: "09:00", End: "17:00", Enabled: false},
		"thu": {Start: "09:00", End: "17:00", Enabled: false},
		"fri": {Start: "09:00", End: "17:00", Enabled: false},
		"sat": {Start: "09:00", End: "17:00", Enabled: false},
		"sun": {Start: "09:00", End: "17:00", Enabled: false},
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := instRepo.Create(ctx, &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   "wk-flow-escalate",
		SecretKey:   "sk-flow-escalate",
		Settings:    string(settingsJSON),
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	svc := &SupportAIService{
		conversationRepo: convRepo,
		messageRepo:      messageRepo,
		handoffRepo:      handoffRepo,
		installationRepo: instRepo,
	}

	if err := svc.EscalateToHuman(ctx, workspaceID, conv.ID, "customer_requested"); err != nil {
		t.Fatalf("EscalateToHuman: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAfterHoursQueue {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAfterHoursQueue)
	}
	// Task 12: with no teammate available (selection == nil), the escalation must
	// still notify the mailbox team that the conversation landed in the queue.
	// That team-facing signal is unconditional: the persisted after_hours handoff
	// state, the after_hours_queue flow state, and the internal escalation system
	// message asserted below — none of which are gated on an assignee being picked
	// — ensure the queued conversation is seen rather than silently discovered.
	if updated.HandoffState == nil || *updated.HandoffState != model.HandoffStateAfterHours {
		t.Fatalf("handoff_state = %#v, want %q", updated.HandoffState, model.HandoffStateAfterHours)
	}
	if updated.AIState == nil || *updated.AIState != "escalated" {
		t.Fatalf("ai_state = %#v, want escalated", updated.AIState)
	}
	if updated.HumanTakeover == nil || !*updated.HumanTakeover {
		t.Fatalf("human_takeover = %#v, want true", updated.HumanTakeover)
	}
	if updated.CustomerRequestedHumanAt == nil {
		t.Fatal("expected customer_requested_human_at to be set")
	}

	messages, err := messageRepo.ListByConversation(ctx, workspaceID, conv.ID, true)
	if err != nil {
		t.Fatalf("ListByConversation: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected escalation reply + system event, got %+v", messages)
	}
	if messages[0].MessageType != "system" || messages[0].SystemEventType == nil || *messages[0].SystemEventType != model.SystemEventCustomerRequestedHuman {
		t.Fatalf("first message = %+v, want customer requested human system event before AI acknowledgement", messages[0])
	}
	if messages[1].MessageType != "reply" || messages[1].SenderType != "ai" {
		t.Fatalf("second message = %+v, want AI acknowledgement after customer requested human event", messages[1])
	}
	var reply, sysEvent *model.SupportMessage
	for i := range messages {
		if messages[i].MessageType == "reply" && !messages[i].IsInternal {
			reply = &messages[i]
		}
		if messages[i].MessageType == "system" && messages[i].IsInternal {
			sysEvent = &messages[i]
		}
	}
	if reply == nil {
		t.Fatalf("expected non-internal AI reply, got %+v", messages)
	}
	if sysEvent == nil {
		t.Fatalf("expected internal system escalation event, got %+v", messages)
	}
	if sysEvent.SystemEventType == nil || *sysEvent.SystemEventType != model.SystemEventCustomerRequestedHuman {
		t.Fatalf("system_event_type = %v, want %q", sysEvent.SystemEventType, model.SystemEventCustomerRequestedHuman)
	}
}

func TestSupportAIServicePublishAIReplySetsAIHandlingFlowState(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-flow-ai"
	seedWorkspace(t, db, workspaceID, "Flow AI WS", "flow-ai-ws", "user-123")

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)

	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "AI handled conversation",
		Status:      "open",
		FlowState:   strPtr(model.SupportConversationFlowStateWaitingForHuman),
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := &SupportAIService{
		conversationRepo: convRepo,
		messageRepo:      messageRepo,
	}

	if _, err := svc.publishAIReply(ctx, workspaceID, conv.ID, "agent-ai", "Here is the answer", "gpt-5", 42, 0.94, nil, "answer", "", "", supportStateProgressing, nil, nil); err != nil {
		t.Fatalf("publishAIReply: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAIHandling {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAIHandling)
	}
	if updated.AIState == nil || *updated.AIState != "pending" {
		t.Fatalf("ai_state = %#v, want pending", updated.AIState)
	}
}

func TestEscalatedConversationFlowStateUsesOfficeHours(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	settings.BusinessHoursEnabled = true
	settings.BusinessHoursTimezone = "UTC"
	settings.BusinessHoursSchedule = map[string]model.BusinessHoursDay{
		"mon": {Start: "09:00", End: "17:00", Enabled: false},
		"tue": {Start: "09:00", End: "17:00", Enabled: false},
		"wed": {Start: "09:00", End: "17:00", Enabled: false},
		"thu": {Start: "09:00", End: "17:00", Enabled: false},
		"fri": {Start: "09:00", End: "17:00", Enabled: false},
		"sat": {Start: "09:00", End: "17:00", Enabled: false},
		"sun": {Start: "09:00", End: "17:00", Enabled: false},
	}

	if got := escalatedConversationFlowState(settings, time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)); got != model.SupportConversationFlowStateAfterHoursQueue {
		t.Fatalf("escalatedConversationFlowState() = %q, want %q", got, model.SupportConversationFlowStateAfterHoursQueue)
	}
}

// TestSupportAIServiceEscalateToHumanHandoffStateFollowsPresence proves that the
// customer-facing handoff_state is derived from real teammate presence (the same
// source the widget's pre-chat availability uses) rather than from whether an
// assignee was selected. With the DEFAULT HandoffBehavior "unassigned",
// selection is always nil, so before this fix a teammate being online still
// produced handoff_state "busy" and the email-capture card, contradicting the
// widget's own "Online now" availability.
func TestSupportAIServiceEscalateToHumanHandoffStateFollowsPresence(t *testing.T) {
	tests := []struct {
		name           string
		teammateOnline bool
		wantHandoff    string
	}{
		{name: "teammate online within hours -> live", teammateOnline: true, wantHandoff: model.HandoffStateLive},
		{name: "nobody online within hours -> busy", teammateOnline: false, wantHandoff: model.HandoffStateBusy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			ensureSupportModuleGrantsTable(t, db)

			const (
				workspaceID = "ws-escalate-presence"
				ownerID     = "user-escalate-owner"
				supportID   = "user-escalate-support"
			)

			seedUser(t, db, ownerID, "owner-escalate@example.com", "Owner Escalate", "hash")
			seedUser(t, db, supportID, "support-escalate@example.com", "Support Escalate", "hash")
			seedWorkspace(t, db, workspaceID, "Escalate Presence WS", "escalate-presence-ws", ownerID)
			seedWorkspaceMember(t, db, "wm-escalate-owner", workspaceID, ownerID, "owner-escalate@example.com", "Owner Escalate", model.RoleAdmin)
			seedWorkspaceMember(t, db, "wm-escalate-support", workspaceID, supportID, "support-escalate@example.com", "Support Escalate", model.RoleMember)
			seedSupportModuleGrant(t, db, "grant-escalate-support", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-escalate-support")

			convRepo := repository.NewSupportConversationRepository(db)
			messageRepo := repository.NewSupportMessageRepository(db)
			handoffRepo := repository.NewAgentHandoffRepository(db)
			instRepo := repository.NewSupportInboxInstallationRepository(db)

			aiPending := "pending"
			conv := &model.SupportConversation{
				WorkspaceID: workspaceID,
				Subject:     "Need a human",
				Status:      "open",
				AIState:     &aiPending,
				FlowState:   strPtr(model.SupportConversationFlowStateAIHandling),
			}
			if err := convRepo.Create(ctx, conv); err != nil {
				t.Fatalf("create conversation: %v", err)
			}

			// DEFAULT handoff behavior ("unassigned") + business hours disabled so
			// we are always WITHIN hours. This isolates the presence signal.
			settings := model.DefaultSupportInboxSettings()
			settings.BusinessHoursEnabled = false
			if strings.TrimSpace(settings.HandoffBehavior) != "unassigned" {
				t.Fatalf("expected default HandoffBehavior to be 'unassigned', got %q", settings.HandoffBehavior)
			}
			settingsJSON, err := json.Marshal(settings)
			if err != nil {
				t.Fatalf("marshal settings: %v", err)
			}
			if err := instRepo.Create(ctx, &model.SupportWidgetInstallation{
				WorkspaceID: workspaceID,
				WidgetKey:   "wk-escalate-presence",
				SecretKey:   "sk-escalate-presence",
				Settings:    string(settingsJSON),
				Active:      true,
			}); err != nil {
				t.Fatalf("create installation: %v", err)
			}

			presence := websocket.NewPresenceState()
			if tt.teammateOnline {
				if _, err := presence.SetAgentOnline(ctx, workspaceID, supportID, "conn-escalate-support"); err != nil {
					t.Fatalf("SetAgentOnline: %v", err)
				}
			}

			svc := &SupportAIService{
				conversationRepo:   convRepo,
				messageRepo:        messageRepo,
				handoffRepo:        handoffRepo,
				installationRepo:   instRepo,
				workspaceRepo:      repository.NewWorkspaceRepository(db),
				statusOverrideRepo: repository.NewSupportTeammateStatusOverrideRepository(db),
				presence:           presence,
			}

			if err := svc.EscalateToHuman(ctx, workspaceID, conv.ID, "customer_requested"); err != nil {
				t.Fatalf("EscalateToHuman: %v", err)
			}

			updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if updated.HandoffState == nil || *updated.HandoffState != tt.wantHandoff {
				t.Fatalf("handoff_state = %#v, want %q", updated.HandoffState, tt.wantHandoff)
			}
		})
	}
}

func TestCreateConversationMessageFirstHumanReplyPreservesMessageProjection(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	// Mirror the message insert projection so an older conversation snapshot
	// cannot silently overwrite the state written when the reply is inserted.
	mustExec(t, db, `CREATE TRIGGER test_reply_projection AFTER INSERT ON support_messages
 WHEN NEW.message_type = 'reply' AND NEW.is_internal = false AND NEW.system_event_type IS NULL
 BEGIN UPDATE support_conversations SET
 list_last_message_id=NEW.id, list_last_message_at=NEW.created_at,
 list_last_message_preview=NEW.content, last_public_message_id=NEW.id,
 last_public_message_at=NEW.created_at,last_public_sender_type=NEW.sender_type,
 support_state_version=support_state_version+1
 WHERE id=NEW.conversation_id; END`)

	workspaceID := "ws-flow-reply"
	userID := "user-flow-reply"
	seedUser(t, db, userID, "agent@example.com", "Agent User", "hash")
	seedWorkspace(t, db, workspaceID, "Flow Reply WS", "flow-reply-ws", userID)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	userRepo := repository.NewUserRepository(db)

	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Escalated conversation",
		Status:      "open",
		FlowState:   strPtr(model.SupportConversationFlowStateWaitingForHuman),
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	customerMessage := &model.SupportMessage{
		WorkspaceID: workspaceID, ConversationID: conv.ID, SenderType: "customer",
		MessageType: "reply", Content: "I still need help.",
	}
	if err := messageRepo.Create(ctx, customerMessage); err != nil {
		t.Fatalf("create customer message: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		nil,
		nil,
		nil,
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

	displayName := "Agent User"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "I can help with this.", MessageType: "reply"},
		"user",
		&userID,
		nil,
		&displayName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if derefString(updated.ListLastMessagePreview) != "I can help with this." || derefString(updated.LastPublicSenderType) != "user" || updated.SupportStateVersion != 2 {
		t.Fatalf("reply projection overwritten: preview=%q sender=%q version=%d", derefString(updated.ListLastMessagePreview), derefString(updated.LastPublicSenderType), updated.SupportStateVersion)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}
	if updated.OpenedByUserID == nil || *updated.OpenedByUserID != userID {
		t.Fatalf("opened_by_user_id = %#v, want %q", updated.OpenedByUserID, userID)
	}
	if updated.HumanTakeover == nil || !*updated.HumanTakeover {
		t.Fatalf("human_takeover = %#v, want true", updated.HumanTakeover)
	}
	if updated.TeamLastSeenAt == nil {
		t.Fatal("team_last_seen_at = nil, want teammate reply to advance the read cursor")
	}
	if updated.UnreadCount != 0 {
		t.Fatalf("unread_count = %d, want 0 after teammate reply", updated.UnreadCount)
	}
}
