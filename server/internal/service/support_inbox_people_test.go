package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

func TestNewSupportResponderIndex_CountsRepliesPerInbox(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	joined := now.Add(-60 * 24 * time.Hour)
	billing := "mailbox-billing"
	var replies []repository.SupportTeammateReply
	for day := 1; day <= 3; day++ {
		repliedAt := now.Add(-time.Duration(day) * 24 * time.Hour)
		replies = append(replies,
			repository.SupportTeammateReply{UserID: "billing-bea", MailboxID: &billing, RepliedAt: repliedAt, MemberCreatedAt: joined},
			repository.SupportTeammateReply{UserID: "shared-sam", RepliedAt: repliedAt, MemberCreatedAt: joined},
		)
	}

	index := newSupportResponderIndex(replies, time.UTC, now)
	if !index.workspace.includes("billing-bea") || !index.workspace.includes("shared-sam") {
		t.Fatal("everyone who answers customers should count across the workspace")
	}
	if !index.inbox(billing).includes("billing-bea") {
		t.Error("a teammate who answers the billing inbox should count for it")
	}
	if index.inbox(billing).includes("shared-sam") {
		t.Error("Shared Inbox replies should not count for the billing inbox")
	}
	if !index.inbox("").includes("shared-sam") || index.inbox("").includes("billing-bea") {
		t.Error("only Shared Inbox replies should count for the Shared Inbox")
	}
	if index.inbox("mailbox-quiet").hasReplyHistory {
		t.Error("an inbox nobody replied in should have no reply history")
	}
}

// inboxAlertFixture is a workspace with a Shared Inbox and team inboxes:
//   - admin-ada is an admin who is online but never answers customers
//   - shared-sam answers customers in the Shared Inbox
//   - billing-bea and billing-ben are Billing members; only Bea answers there
//   - pool-pia is the Sales inbox's "Assign to" person
//   - Empty has no members; Fresh has Ben as a member and no replies yet
type inboxAlertFixture struct {
	db          *gorm.DB
	workspaceID string
	inboxes     map[string]string
	presence    *websocket.PresenceState
	directory   *SupportResponderDirectory
}

func newInboxAlertFixture(t *testing.T) *inboxAlertFixture {
	t.Helper()
	db := newTestDB(t)
	ctx := context.Background()
	ensureSupportModuleGrantsTable(t, db)
	now := time.Now()

	f := &inboxAlertFixture{db: db, workspaceID: "ws-inbox-alerts", inboxes: map[string]string{}, presence: websocket.NewPresenceState()}
	seedUser(t, db, "admin-ada", "ada@example.com", "Ada Admin", "hash")
	seedWorkspace(t, db, f.workspaceID, "Inbox Alerts", "inbox-alerts", "admin-ada")
	seedWorkspaceMember(t, db, "wm-admin-ada", f.workspaceID, "admin-ada", "ada@example.com", "Ada Admin", model.RoleAdmin)
	for _, userID := range []string{"shared-sam", "billing-bea", "billing-ben", "pool-pia"} {
		seedUser(t, db, userID, userID+"@example.com", userID, "hash")
		mustExec(t, db, `INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"wm-"+userID, f.workspaceID, userID, userID+"@example.com", userID, model.RoleMember, model.WorkspaceMemberStatusActive, now.Add(-60*24*time.Hour), now)
		seedSupportModuleGrant(t, db, "grant-"+userID, f.workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-"+userID)
	}
	for _, userID := range []string{"admin-ada", "shared-sam", "billing-bea", "billing-ben", "pool-pia"} {
		mustExec(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"settings-"+userID, userID, true, "immediate", "09:00", 1, false, "all", "UTC", now, now)
	}

	mailboxRepo := repository.NewSupportMailboxRepository(db)
	for _, inbox := range []struct {
		name, mode string
		pool       []string
		members    []string
	}{
		{name: "Billing", mode: "manual", members: []string{"billing-bea", "billing-ben"}},
		{name: "Sales", mode: "round_robin", pool: []string{"wm-pool-pia"}, members: []string{"pool-pia", "billing-ben"}},
		{name: "Empty", mode: "manual"},
		{name: "Fresh", mode: "manual", members: []string{"billing-ben"}},
	} {
		mailbox := &model.SupportMailbox{WorkspaceID: f.workspaceID, Name: inbox.name, Handle: inbox.name, Icon: "inbox", AssignmentMode: inbox.mode, AssignmentMemberIDs: inbox.pool, Active: true, CreatedByID: "admin-ada"}
		if err := mailboxRepo.Create(ctx, mailbox); err != nil {
			t.Fatalf("create %s inbox: %v", inbox.name, err)
		}
		f.inboxes[inbox.name] = mailbox.ID
		for _, userID := range inbox.members {
			mustExec(t, db, `INSERT INTO support_mailbox_memberships (id, mailbox_id, workspace_member_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
				fmt.Sprintf("smm-%s-%s", inbox.name, userID), mailbox.ID, "wm-"+userID, now, now)
		}
	}

	// Sam answers the Shared Inbox and Bea answers Billing, each on three days.
	sharedHistory := f.conversation(t, "")
	billingHistory := f.conversation(t, f.inboxes["Billing"])
	for day := 1; day <= 3; day++ {
		for _, reply := range []struct{ userID, conversationID string }{{"shared-sam", sharedHistory.ID}, {"billing-bea", billingHistory.ID}} {
			mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, sender_user_id, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				fmt.Sprintf("reply-%s-%d", reply.userID, day), f.workspaceID, reply.conversationID, "user", "reply", reply.userID, "hello", false, now.Add(-time.Duration(day)*24*time.Hour), now)
		}
	}

	f.setAround(t, "admin-ada")
	f.directory = NewSupportResponderDirectory(repository.NewSupportMessageRepository(db))
	return f
}

func (f *inboxAlertFixture) conversation(t *testing.T, mailboxID string) *model.SupportConversation {
	t.Helper()
	conv := &model.SupportConversation{WorkspaceID: f.workspaceID, Subject: "Question", Status: "open"}
	if mailboxID != "" {
		conv.MailboxID = &mailboxID
	}
	if err := repository.NewSupportConversationRepository(f.db).Create(context.Background(), conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return conv
}

func (f *inboxAlertFixture) setAround(t *testing.T, userIDs ...string) {
	t.Helper()
	ctx := context.Background()
	for _, userID := range userIDs {
		if _, err := f.presence.SetAgentOnline(ctx, f.workspaceID, userID, "conn-"+userID); err != nil {
			t.Fatalf("SetAgentOnline: %v", err)
		}
		if err := f.presence.TouchAgentActivity(ctx, f.workspaceID, userID, "conn-"+userID); err != nil {
			t.Fatalf("TouchAgentActivity: %v", err)
		}
	}
}

func (f *inboxAlertFixture) alertRecipient(t *testing.T, conv *model.SupportConversation) string {
	t.Helper()
	selection, err := selectSupportReplyAlertRecipient(
		context.Background(),
		repository.NewWorkspaceRepository(f.db),
		repository.NewSupportMailboxRepository(f.db),
		repository.NewSupportMessageRepository(f.db),
		repository.NewSupportInboxInstallationRepository(f.db),
		repository.NewNotificationPreferenceRepository(f.db),
		f.presence,
		repository.NewSupportTeammateStatusOverrideRepository(f.db),
		f.directory,
		conv,
		"support_conversation.customer_reply",
		time.Now(),
	)
	if err != nil {
		t.Fatalf("selectSupportReplyAlertRecipient: %v", err)
	}
	if selection == nil {
		return ""
	}
	return selection.UserID
}

func TestSelectSupportReplyAlertRecipient(t *testing.T) {
	f := newInboxAlertFixture(t)
	tests := []struct {
		name      string
		inbox     string
		assigned  string
		lastReply string
		want      string
	}{
		{name: "the assignee comes first", inbox: "Billing", assigned: "billing-ben", want: "billing-ben"},
		{name: "then the teammate who last replied", inbox: "Billing", lastReply: "billing-ben", want: "billing-ben"},
		{name: "a last replier from the inbox the conversation left is skipped", inbox: "Billing", lastReply: "shared-sam", want: "billing-bea"},
		{name: "a team inbox goes to who answers it, not an online admin", inbox: "Billing", want: "billing-bea"},
		{name: "the inbox's assignment people come before its members", inbox: "Sales", want: "pool-pia"},
		{name: "a new inbox without replies goes to its members", inbox: "Fresh", want: "billing-ben"},
		{name: "an inbox without members goes to admins", inbox: "Empty", want: "admin-ada"},
		{name: "the Shared Inbox goes to who answers customers", want: "shared-sam"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := f.conversation(t, f.inboxes[tt.inbox])
			if tt.assigned != "" {
				conv.AssignedUserID = &tt.assigned
			}
			if tt.lastReply != "" {
				conv.OpenedByUserID = &tt.lastReply
			}
			if got := f.alertRecipient(t, conv); got != tt.want {
				t.Fatalf("alert went to %q, want %q", got, tt.want)
			}
		})
	}
}

// seedMoveAfterReplies has a teammate reply on three days and then moves the
// conversation into its current inbox.
func (f *inboxAlertFixture) seedMoveAfterReplies(t *testing.T, conv *model.SupportConversation, userID string) {
	t.Helper()
	now := time.Now()
	for day := 1; day <= 3; day++ {
		mustExec(t, f.db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, sender_user_id, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("moved-%s-%s-%d", conv.ID, userID, day), f.workspaceID, conv.ID, "user", "reply", userID, "hello", false, now.Add(-time.Duration(day)*24*time.Hour), now)
	}
	mustExec(t, f.db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, system_event_type, content, is_internal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"move-"+conv.ID, f.workspaceID, conv.ID, "user", "system", string(model.SystemEventMailboxMoved), "Sam moved to inbox 'Billing'.", true, now.Add(-time.Hour), now)
}

func TestSupportResponderDirectory_RepliesBeforeAMoveStayWithTheEarlierInbox(t *testing.T) {
	f := newInboxAlertFixture(t)
	f.seedMoveAfterReplies(t, f.conversation(t, f.inboxes["Billing"]), "shared-sam")

	index, err := f.directory.index(context.Background(), f.workspaceID, "UTC", time.Now())
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if index.inbox(f.inboxes["Billing"]).includes("shared-sam") {
		t.Error("replies written before the move into Billing should not make Sam part of the Billing team")
	}
	if !index.inbox(f.inboxes["Billing"]).includes("billing-bea") || !index.workspace.includes("shared-sam") {
		t.Error("other replies should still count")
	}
}

func TestSelectSupportReplyAlertRecipient_SkipsLastReplierAfterAMove(t *testing.T) {
	f := newInboxAlertFixture(t)
	conv := f.conversation(t, f.inboxes["Billing"])
	// Ben is a Billing member, but he replied before the conversation was moved into Billing.
	f.seedMoveAfterReplies(t, conv, "billing-ben")
	lastReplier := "billing-ben"
	conv.OpenedByUserID = &lastReplier
	if got := f.alertRecipient(t, conv); got != "billing-bea" {
		t.Fatalf("alert went to %q, want the Billing team (billing-bea)", got)
	}
}

func TestSupportAIServiceEscalateToHuman_InboxTeamDecidesHandoffState(t *testing.T) {
	tests := []struct {
		name   string
		inbox  string
		around []string
		want   string
	}{
		{name: "someone who answers another inbox is around", inbox: "Billing", around: []string{"shared-sam"}, want: model.HandoffStateAfterHours},
		{name: "someone who answers this inbox is around", inbox: "Billing", around: []string{"billing-bea"}, want: model.HandoffStateLive},
		{name: "a team inbox teammate does not count for the Shared Inbox", around: []string{"billing-bea"}, want: model.HandoffStateAfterHours},
		{name: "someone who answers the Shared Inbox is around", around: []string{"shared-sam"}, want: model.HandoffStateLive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newInboxAlertFixture(t)
			f.setAround(t, tt.around...)
			// Closed every day: the workspace is always outside business hours.
			seedSupportInstallationSettings(t, f.db, f.workspaceID, func(settings *model.SupportInboxSettings) {
				settings.BusinessHoursEnabled = true
				for day, hours := range settings.BusinessHoursSchedule {
					hours.Enabled = false
					settings.BusinessHoursSchedule[day] = hours
				}
			})
			conv := f.conversation(t, f.inboxes[tt.inbox])
			convRepo := repository.NewSupportConversationRepository(f.db)
			if err := convRepo.UpdateFields(context.Background(), f.workspaceID, conv.ID, map[string]any{"ai_state": "pending", "flow_state": model.SupportConversationFlowStateAIHandling}); err != nil {
				t.Fatalf("set AI state: %v", err)
			}

			svc := newInboxAlertAIService(f)
			if err := svc.EscalateToHuman(context.Background(), f.workspaceID, conv.ID, "customer_requested"); err != nil {
				t.Fatalf("EscalateToHuman: %v", err)
			}
			updated, err := convRepo.GetByID(context.Background(), f.workspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if got := derefString(updated.HandoffState); got != tt.want {
				t.Fatalf("handoff_state = %q, want %q", got, tt.want)
			}
		})
	}
}

func newInboxAlertAIService(f *inboxAlertFixture) *SupportAIService {
	messageRepo := repository.NewSupportMessageRepository(f.db)
	svc := &SupportAIService{
		conversationRepo: repository.NewSupportConversationRepository(f.db),
		messageRepo:      messageRepo,
		handoffRepo:      repository.NewAgentHandoffRepository(f.db),
		installationRepo: repository.NewSupportInboxInstallationRepository(f.db),
	}
	svc.SetSupportRoutingDependencies(repository.NewWorkspaceRepository(f.db), f.presence, repository.NewSupportTeammateStatusOverrideRepository(f.db))
	svc.SetSupportAvailabilitySources(f.directory, repository.NewSupportInboxSessionRepository(f.db))
	svc.SetMailboxRepository(repository.NewSupportMailboxRepository(f.db))
	return svc
}
