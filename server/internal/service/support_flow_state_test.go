package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
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
	if updated.AIState == nil || *updated.AIState != "escalated" {
		t.Fatalf("ai_state = %#v, want escalated", updated.AIState)
	}
	if updated.CustomerRequestedHumanAt == nil {
		t.Fatal("expected customer_requested_human_at to be set")
	}

	messages, err := messageRepo.ListByConversation(ctx, workspaceID, conv.ID, true)
	if err != nil {
		t.Fatalf("ListByConversation: %v", err)
	}
	if len(messages) != 1 || messages[0].MessageType != "system" {
		t.Fatalf("expected one system escalation message, got %+v", messages)
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

	if _, err := svc.publishAIReply(ctx, workspaceID, conv.ID, "agent-ai", "Here is the answer", "gpt-5", 42, 0.94, nil, "answer"); err != nil {
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
