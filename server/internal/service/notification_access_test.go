package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type notificationAccessFunc func(context.Context, string, model.NotificationEventInput) (bool, error)

func TestAskAgentNotificationAccessIsOwnerScopedWithoutAutomation(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	for _, stmt := range []string{
		`CREATE TABLE agent_runs (id TEXT, workspace_id TEXT, dock_chat_id TEXT)`,
		`CREATE TABLE dock_chats ( flow_builder TEXT,id TEXT, workspace_id TEXT, user_id TEXT, archived_at DATETIME)`,
		`INSERT INTO agent_runs VALUES ('run','ws','chat')`,
		`INSERT INTO dock_chats VALUES ('chat','ws','owner',NULL)`,
	} {
		mustExecNotificationService(t, db, stmt)
	}
	members := &notificationAuditMembers{status: "active", role: "member"}
	policy := NewNotificationAccessPolicy(authorization.NewAuthzService(db, members, &notificationAuditModules{}), repository.NewNotificationRepository(db))
	event := model.NotificationEventInput{WorkspaceID: "ws", EntityType: "agent_run", EntityID: "run", Metadata: model.JSONB{"dock_chat_id": "forged-chat", "task_id": "forged-task"}}
	check := func(user string, want bool) {
		t.Helper()
		allowed, err := policy.CanReceive(context.Background(), user, event)
		if err != nil || allowed != want {
			t.Fatalf("user %s: allowed=%v err=%v, want %v", user, allowed, err, want)
		}
	}
	check("owner", true)
	check("other", false)
	members.role = "admin"
	check("other", false)
	event.WorkspaceID = "elsewhere"
	check("owner", false)
	event.WorkspaceID = "ws"
	members.status = "revoked"
	check("owner", false)
	members.status = "active"
	mustExecNotificationService(t, db, `UPDATE dock_chats SET archived_at = CURRENT_TIMESTAMP`)
	check("owner", false)
}

func (f notificationAccessFunc) CanReceive(ctx context.Context, user string, event model.NotificationEventInput) (bool, error) {
	return f(ctx, user, event)
}

func TestNotificationAccessRevocationFiltersInboxCountAndDigest(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	now := time.Now()
	seedNotificationServiceUser(t, db, "user-1", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "daily", nil, now)
	repo := repository.NewNotificationRepository(db)
	svc := NewNotificationService(repo, repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, repository.NewUserRepository(db), nil, nil, &stubEmailSender{}, "")
	allowed := true
	svc.SetAccessChecker(notificationAccessFunc(func(context.Context, string, model.NotificationEventInput) (bool, error) { return allowed, nil }))
	event := model.NotificationEventInput{WorkspaceID: "ws", EntityType: "task", EntityID: "task", EventType: "task.assigned", TeamID: "team", ExplicitRecipients: []string{"user-1"}, SkipFollowers: true}
	if err := svc.Emit(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	count, err := svc.UnreadCount(context.Background(), "user-1", "ws")
	if err != nil || count != 1 {
		t.Fatalf("before revoke: %d %v", count, err)
	}
	allowed = false
	page, err := svc.List(context.Background(), "user-1", "ws", "", "", 20, nil)
	if err != nil || len(page.Data) != 0 || page.UnreadCount != 0 {
		t.Fatalf("after revoke: %+v %v", page, err)
	}
	deliveries, err := repo.ListPendingDigestDeliveries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 1 || notificationTeam(deliveries[0].EventMetadata) != "team" {
		t.Fatalf("lost delivery scope: %+v", deliveries)
	}
	kept, skipped, err := svc.filterDigestDeliveriesByCurrentPreferences(context.Background(), "user-1", deliveries)
	if err != nil || len(kept) != 0 || len(skipped) != 1 {
		t.Fatalf("queued email after revoke: %v %v %v", kept, skipped, err)
	}
	event.EntityID = "other"
	if err := svc.Emit(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	assertNotificationServiceCount(t, db, "notifications", 1)
}

func TestDigestRechecksTeamOverride(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	now := time.Now()
	ctx := context.Background()
	seedNotificationServiceUserSettings(t, db, "user-1", true, "daily", nil, now)
	mustExecNotificationService(t, db, `INSERT INTO notification_preferences (id,user_id,workspace_id,team_id,mute_workspace,channel_preferences) VALUES (?,?,?,?,?,?)`, "p", "user-1", "ws", "team", false, `{"comments":{"email":false}}`)
	svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, nil, nil, nil, nil, "")
	kept, skipped, err := svc.filterDigestDeliveriesByCurrentPreferences(ctx, "user-1", []repository.PendingDigestDelivery{{DeliveryID: "d", WorkspaceID: "ws", EventType: "comment.created", EventMetadata: model.JSONB{notificationTeamKey: "team"}}})
	if err != nil || len(kept) != 0 || len(skipped) != 1 {
		t.Fatalf("team override ignored: %v %v %v", kept, skipped, err)
	}
}

type notificationAuditMembers struct {
	status string
	role   string
	teams  []authorization.TeamRole
}

func (r *notificationAuditMembers) GetMembership(context.Context, string, string) (*authorization.MemberInfo, error) {
	return &authorization.MemberInfo{ID: "member", Role: r.role, Status: r.status}, nil
}
func (r *notificationAuditMembers) GetTeamMemberships(context.Context, string) ([]authorization.TeamRole, error) {
	return r.teams, nil
}

type notificationAuditModules struct{ direct bool }

func (r *notificationAuditModules) ListAccessibleModules(_ context.Context, _ string, _ string, teams []string) ([]model.ModuleID, error) {
	if r.direct {
		return []model.ModuleID{model.ModuleCRM, model.ModuleSupport}, nil
	}
	for _, team := range teams {
		if team == "granted" {
			return []model.ModuleID{model.ModuleCRM, model.ModuleSupport}, nil
		}
	}
	return nil, nil
}

func TestNotificationAccessPolicyUsesMemberAndTeamGrantsAndCurrentResourceScope(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`CREATE TABLE crm_signals (id TEXT,workspace_id TEXT)`,
		`CREATE TABLE pm_tasks (id TEXT,workspace_id TEXT,team_id TEXT)`,
		`CREATE TABLE docs_documents (id TEXT,workspace_id TEXT,space_id TEXT,deleted_at DATETIME)`,
		`CREATE TABLE docs_spaces (id TEXT,workspace_id TEXT,visibility TEXT,deleted_at DATETIME)`,
		`CREATE TABLE docs_space_teams (space_id TEXT,team_id TEXT)`,
		`INSERT INTO crm_signals VALUES ('signal','ws')`,
		`INSERT INTO pm_tasks VALUES ('task','ws','granted')`,
		`INSERT INTO docs_documents VALUES ('doc','ws','space',NULL)`,
		`INSERT INTO docs_spaces VALUES ('space','ws','team_only',NULL)`,
		`INSERT INTO docs_space_teams VALUES ('space','granted')`,
	} {
		mustExecNotificationService(t, db, stmt)
	}
	members := &notificationAuditMembers{status: "active", role: "member"}
	modules := &notificationAuditModules{}
	policy := NewNotificationAccessPolicy(authorization.NewAuthzService(db, members, modules), repository.NewNotificationRepository(db))
	check := func(entity, id string, want bool) {
		t.Helper()
		got, err := policy.CanReceive(ctx, "user", model.NotificationEventInput{WorkspaceID: "ws", EntityType: entity, EntityID: id})
		if err != nil || got != want {
			t.Fatalf("%s: got %v %v, want %v", entity, got, err, want)
		}
	}
	check("crm_signal", "signal", false)
	modules.direct = true
	check("crm_signal", "signal", true)
	modules.direct = false
	members.teams = []authorization.TeamRole{{TeamID: "granted", Role: "member"}}
	check("crm_signal", "signal", true)
	check("task", "task", true)
	check("doc", "doc", true)
	mustExecNotificationService(t, db, `UPDATE pm_tasks SET team_id='other'`)
	check("task", "task", false)
	members.teams = nil
	check("crm_signal", "signal", false)
	check("doc", "doc", false)
	members.role = "admin"
	check("crm_signal", "signal", true)
	check("task", "task", true)
	check("doc", "doc", true)
	members.status = "revoked"
	check("crm_signal", "signal", false)
}

func TestSupportMentionPushHonorsPauseMuteCategoryAndAccess(t *testing.T) {
	for _, blocked := range []string{"none", "pause", "mute", "category", "access"} {
		t.Run(blocked, func(t *testing.T) {
			db := newNotificationServiceTestDB(t)
			ctx := context.Background()
			now := time.Now()
			seedNotificationServiceUserSettings(t, db, "user", true, "daily", nil, now)
			if blocked == "pause" {
				mustExecNotificationService(t, db, `UPDATE user_notification_settings SET do_not_disturb=true`)
			}
			prefs := `{}`
			if blocked == "category" {
				prefs = `{"support_mentions":{"in_app":false}}`
			}
			mustExecNotificationService(t, db, `INSERT INTO notification_preferences (id,user_id,workspace_id,mute_workspace,channel_preferences) VALUES (?,?,?,?,?)`, "p", "user", "ws", blocked == "mute", prefs)
			svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, nil, nil, nil, nil, "")
			svc.SetAccessChecker(notificationAccessFunc(func(context.Context, string, model.NotificationEventInput) (bool, error) {
				return blocked != "access", nil
			}))
			client := &fakeFCMClient{}
			push := NewPushSenderService(&fakePushSenderRepo{devices: []model.PushDevice{{UserID: "user", Token: "test"}}}, client)
			svc.sendSupportPush(ctx, push, []string{"user"}, &model.SupportConversation{ID: "conv", WorkspaceID: "ws"}, "support_conversation.mentioned", "", PushNotification{Title: "Mention"})
			want := 0
			if blocked == "none" {
				want = 1
			}
			if len(client.sent) != want {
				t.Fatalf("sent %d pushes, want %d", len(client.sent), want)
			}
		})
	}
}

func TestEmailOnlySupportReplyRespectsReadBeforeDelay(t *testing.T) {
	for _, read := range []bool{false, true} {
		t.Run(fmt.Sprint(read), func(t *testing.T) {
			db := newNotificationServiceTestDB(t)
			ctx := context.Background()
			now := time.Now()
			seedNotificationServiceUser(t, db, "user", "user@example.com", "User", now)
			seedNotificationServiceUserSettings(t, db, "user", true, "daily", nil, now)
			mustExecNotificationService(t, db, `INSERT INTO notification_preferences (id,user_id,workspace_id,mute_workspace,channel_preferences) VALUES (?,?,?,?,?)`, "p", "user", "ws", false, `{"support_replies":{"in_app":false,"email":true}}`)
			repo := repository.NewNotificationRepository(db)
			emailer := &stubEmailSender{}
			svc := NewNotificationService(repo, repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, repository.NewUserRepository(db), nil, nil, emailer, "")
			event := model.NotificationEventInput{WorkspaceID: "ws", EntityType: "support_conversation", EntityID: "conv", EventType: "support_conversation.customer_reply", Category: model.NotifCategorySupportReplies, Priority: "high", ExplicitRecipients: []string{"user"}, DelayedEmailChannel: "support_reply_email"}
			if err := svc.Emit(ctx, event); err != nil {
				t.Fatal(err)
			}
			if read {
				if err := svc.MarkEntityCategoryAsRead(ctx, "user", "ws", "support_conversation", "conv", model.NotifCategorySupportReplies); err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.ProcessPendingSupportReplyEmails(ctx, now.Add(4*time.Minute)); err != nil {
				t.Fatal(err)
			}
			want := 1
			if read {
				want = 0
			}
			if len(emailer.sent) != want {
				t.Fatalf("sent %d emails, want %d", len(emailer.sent), want)
			}
			page, err := svc.List(ctx, "user", "ws", "", "", 20, nil)
			if err != nil || len(page.Data) != 0 || page.UnreadCount != 0 {
				t.Fatalf("email-only reply leaked into inbox: %+v %v", page, err)
			}
		})
	}
}

func TestNotificationUnreadCountInitializesTimezoneOnce(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	mustExecNotificationService(t, db, "CREATE UNIQUE INDEX notification_settings_user_unique ON user_notification_settings(user_id)")
	settingsRepo := repository.NewUserNotificationSettingsRepository(db)
	svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), settingsRepo, nil, nil, nil, nil, nil, "")
	ctx := context.Background()
	for _, hint := range []string{"Asia/Karachi", "America/New_York"} {
		if _, err := svc.UnreadCount(ctx, "user-1", "ws", hint); err != nil {
			t.Fatal(err)
		}
		settings, err := settingsRepo.Get(ctx, "user-1")
		if err != nil || settings.Timezone != "Asia/Karachi" {
			t.Fatalf("timezone not preserved: %+v %v", settings, err)
		}
	}
}
