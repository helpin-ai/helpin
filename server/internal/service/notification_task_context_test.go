package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestTaskRecipientEventExplainsRelationship(t *testing.T) {
	task := &repository.TaskNotificationContext{Title: "Fix billing", RequesterID: "requester", StateName: "In Progress", Owners: []repository.TaskNotificationOwner{{UserID: "owner", Name: "Alex"}}}
	tests := []struct {
		name, kind, recipient, want string
		follows                     bool
		explicit                    []string
	}{
		{"requester status", "task.status_changed", "requester", "Waleed moved Fix billing you requested to In Progress", false, nil},
		{"owner comment", "comment.created", "owner", "Waleed commented on Fix billing assigned to you", true, nil},
		{"follower update", "task.updated", "follower", "Waleed updated Fix billing you follow", true, nil},
		{"direct assignment", "task.assigned", "owner", "Waleed assigned you to Fix billing", true, []string{"owner"}},
		{"requester sees another assignee", "task.assigned", "requester", "Waleed assigned Fix billing you requested to Alex", false, []string{"owner"}},
		{"direct mention", "comment.mention", "requester", "Waleed mentioned you in a comment on Fix billing you requested", false, []string{"requester"}},
		{"follower is not mentioned", "comment.mention", "follower", "Waleed added a mention in Fix billing you follow", true, []string{"requester"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := model.NotificationEventInput{EntityType: "task", EventType: tt.kind, Title: "original", ActorSnapshot: model.JSONB{"name": "Waleed"}, Metadata: model.JSONB{"original": true}, ExplicitRecipients: tt.explicit}
			got := taskRecipientEvent(event, task, tt.recipient, tt.follows)
			if got.Title != tt.want {
				t.Fatalf("title = %q, want %q", got.Title, tt.want)
			}
			if got.EntitySnapshot["title"] != "Fix billing" {
				t.Fatal("missing task title for links")
			}
			if event.Title != "original" || len(event.Metadata) != 1 {
				t.Fatal("recipient personalization modified the shared event")
			}
			if got.Metadata[notificationReasonKey] == "" {
				t.Fatal("missing relationship explanation")
			}
			if immediateEmailActionText(got) != strings.TrimPrefix(tt.want, "Waleed ") {
				t.Fatal("immediate email lost personalized wording")
			}
		})
	}
}

func TestEmitTaskContextPersonalizesEachRecipient(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme", now)
	for _, id := range []string{"requester", "owner", "follower"} {
		seedNotificationServiceUser(t, db, id, id+"@example.com", id, now)
		seedNotificationServiceUserSettings(t, db, id, true, "daily", nil, now)
	}
	for _, query := range []string{
		`CREATE TABLE workspace_members (id TEXT, workspace_id TEXT, user_id TEXT)`,
		`CREATE TABLE pm_workflow_states (id TEXT, name TEXT)`,
		`CREATE TABLE pm_tasks (id TEXT, workspace_id TEXT, name TEXT, requester_id TEXT, requester_member_id TEXT, workflow_state_id TEXT)`,
		`CREATE TABLE pm_task_owners (task_id TEXT, user_id TEXT)`,
		`INSERT INTO workspace_members VALUES ('requester-member','ws-1','requester')`,
		`INSERT INTO pm_workflow_states VALUES ('progress','In Progress')`,
		`INSERT INTO pm_tasks VALUES ('task-1','ws-1','Fix billing',NULL,'requester-member','progress')`,
		`INSERT INTO pm_task_owners VALUES ('task-1','owner')`,
	} {
		mustExecNotificationService(t, db, query)
	}
	for _, id := range []string{"owner", "follower"} {
		mustExecNotificationService(t, db, `INSERT INTO entity_followers (id,user_id,entity_type,entity_id,workspace_id,reason,created_at) VALUES (?,?,?,?,?,?,?)`, id, id, "task", "task-1", "ws-1", "manual", now)
	}
	repo := repository.NewNotificationRepository(db)
	svc := NewNotificationService(repo, repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), repository.NewFollowerRepository(db), repository.NewUserRepository(db), repository.NewWorkspaceRepository(db), nil, &stubEmailSender{}, "https://app.helpin.ai")
	err := svc.Emit(ctx, model.NotificationEventInput{WorkspaceID: "ws-1", ActorID: "actor", ActorSnapshot: model.JSONB{"name": "Waleed"}, EntityType: "task", EntityID: "task-1", EventType: "task.status_changed", Title: "Task moved: Fix billing", Category: model.NotifCategoryStatusChanges})
	if err != nil {
		t.Fatal(err)
	}
	var notifications []model.Notification
	if err := db.Find(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if len(notifications) != 3 {
		t.Fatalf("want requester, owner and follower, got %d notifications", len(notifications))
	}
	wants := map[string]string{"requester": "Waleed moved Fix billing you requested to In Progress", "owner": "Waleed moved Fix billing assigned to you to In Progress", "follower": "Waleed moved Fix billing you follow to In Progress"}
	for _, n := range notifications {
		if n.Title != wants[n.RecipientID] {
			t.Errorf("%s: %q", n.RecipientID, n.Title)
		}
	}
	pending, err := repo.ListPendingDigestDeliveries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	items, _, _ := buildDigestItems(pending, now.Add(time.Hour))
	if len(items) != 3 {
		t.Fatalf("want 3 digest items, got %d", len(items))
	}
	for _, item := range items {
		if !strings.Contains(item.Title, " to In Progress") || item.EntityTitle != "Fix billing" {
			t.Fatalf("digest lost event context: %+v", item)
		}
	}
	// Access checks must still apply to the automatically included requester.
	svc.SetAccessChecker(notificationAccessFunc(func(_ context.Context, user string, _ model.NotificationEventInput) (bool, error) {
		return user != "requester", nil
	}))
	if err := svc.Emit(ctx, model.NotificationEventInput{WorkspaceID: "ws-1", ActorID: "actor", ActorSnapshot: model.JSONB{"name": "Waleed"}, EntityType: "task", EntityID: "task-1", EventType: "task.updated", Title: "updated Fix billing", Category: model.NotifCategorySubscriptions}); err != nil {
		t.Fatal(err)
	}
	var requesterNotification model.Notification
	if err := db.Where("recipient_id = ?", "requester").First(&requesterNotification).Error; err != nil {
		t.Fatal(err)
	}
	if requesterNotification.EventCount != 1 {
		t.Fatal("requester received an update after access was denied")
	}

	other, err := repo.TaskNotificationContext(ctx, "other-workspace", "task-1")
	if err != nil || other != nil {
		t.Fatalf("task context crossed workspace scope: %+v %v", other, err)
	}
}

func TestTaskRecipientImmediateEmailPreservesContext(t *testing.T) {
	event := taskRecipientEvent(model.NotificationEventInput{EntityType: "task", EntityID: "task-1", EventType: "comment.created", Category: model.NotifCategoryComments, ActorSnapshot: model.JSONB{"name": "Waleed"}}, &repository.TaskNotificationContext{Title: "Fix billing", RequesterID: "requester"}, "requester", false)
	svc := NewNotificationService(nil, nil, nil, nil, nil, nil, nil, nil, "https://app.helpin.ai")
	subject, htmlBody, textBody := svc.renderImmediateEmail(context.Background(), event)
	if !strings.Contains(htmlBody, "you requested") || !strings.Contains(textBody, "Waleed commented on Fix billing you requested") {
		t.Fatal("immediate email lost requester context")
	}
	if strings.Contains(subject, "Waleed Waleed") {
		t.Fatal("actor repeated in subject")
	}
	if strings.Contains(htmlBody, "a task assigned to you") {
		t.Fatal("requester described as assignee")
	}
}
