package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCreateConversationMessage_CustomerReplyFallsBackToWorkspaceRecipient(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now()

	workspaceID := "ws-support-fallback"
	userA := "user-a"
	userB := "user-b"

	seedUser(t, db, userA, "a@example.com", "User A", "hash")
	seedUser(t, db, userB, "b@example.com", "User B", "hash")
	seedWorkspace(t, db, workspaceID, "Support Fallback WS", "support-fallback-ws", userA)
	seedWorkspaceMember(t, db, "wm-a", workspaceID, userA, "a@example.com", "User A", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-b", workspaceID, userB, "b@example.com", "User B", model.RoleMember)
	seedSupportInstallationSettings(t, db, workspaceID, func(settings *model.SupportInboxSettings) {
		settings.HandoffBehavior = "round_robin"
	})

	for _, userID := range []string{userA, userB} {
		mustExec(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"settings-"+userID, userID, true, "immediate", "09:00", 1, false, "all", "UTC", now, now)
	}

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	installRepo := repository.NewSupportInboxInstallationRepository(db)

	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Billing question",
		Status:      "open",
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	emailer := &stubEmailSender{}
	notificationService := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	).SetSupportRoutingDependencies(installRepo, nil, repository.NewSupportTeammateStatusOverrideRepository(db))

	svc := NewSupportInboxService(
		convRepo,
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		installRepo,
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)
	svc.SetNotificationService(notificationService, repository.NewWorkspaceRepository(db))

	customerName := "Customer"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "I still need help with billing", MessageType: "reply"},
		"customer",
		nil,
		nil,
		&customerName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	var notifications []model.Notification
	if err := db.WithContext(ctx).Order("recipient_id ASC").Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}
	if notifications[0].RecipientID != userA {
		t.Fatalf("recipient_id = %q, want %q", notifications[0].RecipientID, userA)
	}
}

func TestSupportAIServiceEscalateToHumanAssignsAvailableTeamRecipient(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-escalate-team"
	ownerID := "user-owner"
	teammateID := "user-team"
	teamID := "team-support"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hash")
	seedUser(t, db, teammateID, "team@example.com", "Team User", "hash")
	seedWorkspace(t, db, workspaceID, "Escalate Team WS", "escalate-team-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-owner", workspaceID, ownerID, "owner@example.com", "Owner User", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-team", workspaceID, teammateID, "team@example.com", "Team User", model.RoleMember)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		teamID, workspaceID, "Support", "support", time.Now(), time.Now())
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-team", teamID, "wm-team", "member", time.Now(), time.Now())

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	handoffRepo := repository.NewAgentHandoffRepository(db)
	instRepo := repository.NewSupportInboxInstallationRepository(db)
	seedSupportInstallationSettings(t, db, workspaceID, func(settings *model.SupportInboxSettings) {
		settings.BusinessHoursEnabled = false
		settings.HandoffBehavior = "assign_to_team"
		settings.HandoffTeamID = strPtr(teamID)
	})

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

	svc := &SupportAIService{
		conversationRepo: convRepo,
		messageRepo:      messageRepo,
		handoffRepo:      handoffRepo,
		installationRepo: instRepo,
	}
	svc.SetSupportRoutingDependencies(repository.NewWorkspaceRepository(db), nil, repository.NewSupportTeammateStatusOverrideRepository(db))

	if err := svc.EscalateToHuman(ctx, workspaceID, conv.ID, "customer_requested"); err != nil {
		t.Fatalf("EscalateToHuman: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}
	if updated.OpenedByUserID == nil || *updated.OpenedByUserID != teammateID {
		t.Fatalf("opened_by_user_id = %#v, want %q", updated.OpenedByUserID, teammateID)
	}
}

func seedSupportInstallationSettings(t *testing.T, db *gorm.DB, workspaceID string, mutate func(*model.SupportInboxSettings)) {
	t.Helper()

	settings := model.DefaultSupportInboxSettings()
	if mutate != nil {
		mutate(&settings)
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}

	now := time.Now()
	mustExec(t, db, `INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, settings, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"install-"+workspaceID, workspaceID, "wk-"+workspaceID, "sk-"+workspaceID, string(raw), true, now, now)
}
