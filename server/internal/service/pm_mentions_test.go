package service

import (
	"context"
	"sort"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	pmMentionBodyHTML            = "<p>@eng @alice.eng @carol.admin @design @admin.actor</p>"
	pmMentionBodyText            = "@eng @alice.eng @carol.admin @design @admin.actor"
	pmCommentMentionFollowerText = "@eng @admin.actor"
)

type pmMentionTestEnv struct {
	db          *gorm.DB
	workspaceID string

	actorUserID  string
	aliceUserID  string
	bobUserID    string
	carolUserID  string
	daveUserID   string
	collisionUID string

	actorMemberID string
	aliceMemberID string
	bobMemberID   string
	carolMemberID string
	daveMemberID  string

	engTeamID    string
	designTeamID string

	workflowID string
	stateID    string

	workspaceRepo *repository.WorkspaceRepository

	storyService     *PMTaskService
	commentService   *PMCommentService
	checklistService *PMChecklistItemService
	epicService      *PMEpicService
	sprintService    *PMSprintService
	objectiveService *PMObjectiveService
}

func newPMMentionTestEnv(t *testing.T) *pmMentionTestEnv {
	t.Helper()

	db := newTestDB(t)
	now := time.Now().UTC()

	env := &pmMentionTestEnv{
		db:            db,
		workspaceID:   "ws-mentions",
		actorUserID:   "user-actor",
		aliceUserID:   "user-alice",
		bobUserID:     "user-bob",
		carolUserID:   "user-carol",
		daveUserID:    "user-dave",
		collisionUID:  "user-eng",
		actorMemberID: "member-actor",
		aliceMemberID: "member-alice",
		bobMemberID:   "member-bob",
		carolMemberID: "member-carol",
		daveMemberID:  "member-dave",
		engTeamID:     "team-eng",
		designTeamID:  "team-design",
		workflowID:    "workflow-mentions",
		stateID:       "state-mentions",
	}

	seedUser(t, db, env.actorUserID, "actor@example.com", "Admin Actor", "hash")
	seedUser(t, db, env.aliceUserID, "alice@example.com", "Alice Eng", "hash")
	seedUser(t, db, env.bobUserID, "bob@example.com", "Bob Eng", "hash")
	seedUser(t, db, env.carolUserID, "carol@example.com", "Carol Admin", "hash")
	seedUser(t, db, env.daveUserID, "dave@example.com", "Dave Design", "hash")
	seedUser(t, db, env.collisionUID, "eng@example.com", "Eng", "hash")

	seedWorkspace(t, db, env.workspaceID, "Mentions Workspace", "mentions-ws", env.actorUserID)
	seedWorkspaceMember(t, db, env.actorMemberID, env.workspaceID, env.actorUserID, "actor@example.com", "Admin Actor", model.RoleAdmin)
	seedWorkspaceMember(t, db, env.aliceMemberID, env.workspaceID, env.aliceUserID, "alice@example.com", "Alice Eng", model.RoleMember)
	seedWorkspaceMember(t, db, env.bobMemberID, env.workspaceID, env.bobUserID, "bob@example.com", "Bob Eng", model.RoleMember)
	seedWorkspaceMember(t, db, env.carolMemberID, env.workspaceID, env.carolUserID, "carol@example.com", "Carol Admin", model.RoleAdmin)
	seedWorkspaceMember(t, db, env.daveMemberID, env.workspaceID, env.daveUserID, "dave@example.com", "Dave Design", model.RoleMember)
	mustExec(t, db, `INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"member-eng", env.workspaceID, env.collisionUID, "eng@example.com", "Eng", model.RoleMember, "active", now, now)

	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, team_type, default_story_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		env.engTeamID, env.workspaceID, "Engineering", "eng", "engineering", model.PMTaskTypeFeature, now, now)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, team_type, default_story_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		env.designTeamID, env.workspaceID, "Design", "design", "design", model.PMTaskTypeFeature, now, now)

	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-alice-eng", env.engTeamID, env.aliceMemberID, "member", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-bob-eng", env.engTeamID, env.bobMemberID, "member", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-dave-design", env.designTeamID, env.daveMemberID, "member", now, now)

	seedWorkflow(t, db, env.workflowID, env.workspaceID, env.stateID)
	for _, userID := range []string{env.actorUserID, env.aliceUserID, env.bobUserID, env.carolUserID, env.daveUserID, env.collisionUID} {
		mustExec(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"settings-"+userID, userID, false, "daily", "09:00", 1, false, "all", "UTC", now, now)
	}

	env.workspaceRepo = repository.NewWorkspaceRepository(db)
	notifService := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		env.workspaceRepo,
		nil,
		nil,
		"",
	)
	activityService := NewPMActivityService(repository.NewPMActivityRepository(db))

	env.storyService = NewPMTaskService(
		repository.NewPMTaskRepository(db),
		env.workspaceRepo,
		repository.NewPMWorkflowRepository(db),
		repository.NewPMEpicRepository(db),
		repository.NewPMSprintRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewPMChecklistItemRepository(db),
		repository.NewPMExternalLinkRepository(db),
		repository.NewPMAttachmentRepository(db),
		activityService,
		nil,
		nil,
		notifService,
		nil,
	)
	env.commentService = NewPMCommentService(
		repository.NewPMCommentRepository(db),
		repository.NewPMTaskRepository(db),
		nil,
		activityService,
		nil,
		notifService,
		env.workspaceRepo,
	)
	env.checklistService = NewPMChecklistItemService(
		repository.NewPMChecklistItemRepository(db),
		repository.NewPMTaskRepository(db),
		nil,
		notifService,
		env.workspaceRepo,
	)
	env.epicService = NewPMEpicService(
		repository.NewPMEpicRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewGitRepositoryRepository(db),
		repository.NewPMAttachmentRepository(db),
		env.workspaceRepo,
		activityService,
		nil,
		notifService,
	)
	env.sprintService = NewPMSprintService(
		repository.NewPMSprintRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewPMAttachmentRepository(db),
		env.workspaceRepo,
		repository.NewSettingsRepository(db),
		activityService,
		nil,
		notifService,
	)
	env.objectiveService = NewPMObjectiveService(
		repository.NewPMObjectiveRepository(db),
		repository.NewPMKeyResultRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewPMAttachmentRepository(db),
		env.workspaceRepo,
		activityService,
		nil,
		notifService,
	)

	return env
}

func (e *pmMentionTestEnv) expectedRecipientIDs() []string {
	ids := []string{e.aliceUserID, e.bobUserID, e.carolUserID}
	sort.Strings(ids)
	return ids
}

func (e *pmMentionTestEnv) addCarolToDesignTeam(t *testing.T) {
	t.Helper()

	now := time.Now().UTC()
	mustExec(t, e.db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-carol-design", e.designTeamID, e.carolMemberID, "member", now, now)
}

func (e *pmMentionTestEnv) createStory(t *testing.T, description *string) *model.TaskDetail {
	t.Helper()

	teamID := e.engTeamID
	story, err := e.storyService.Create(context.Background(), model.CreateTaskRequest{
		WorkspaceID:     e.workspaceID,
		Name:            "Mention Story",
		Description:     description,
		WorkflowID:      e.workflowID,
		WorkflowStateID: e.stateID,
		TeamID:          &teamID,
	}, e.actorUserID)
	if err != nil {
		t.Fatalf("create story: %v", err)
	}
	return story
}

func (e *pmMentionTestEnv) notificationsFor(t *testing.T, entityType, entityID string) []model.Notification {
	t.Helper()

	var notifications []model.Notification
	if err := e.db.
		Where("workspace_id = ? AND entity_type = ? AND entity_id = ?", e.workspaceID, entityType, entityID).
		Order("recipient_id ASC").
		Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	return notifications
}

func assertMentionNotifications(t *testing.T, notifications []model.Notification, eventType string, wantRecipients []string) {
	t.Helper()

	if len(notifications) != len(wantRecipients) {
		t.Fatalf("notification count = %d, want %d", len(notifications), len(wantRecipients))
	}

	gotRecipients := make([]string, 0, len(notifications))
	for _, notification := range notifications {
		gotRecipients = append(gotRecipients, notification.RecipientID)
		if notification.EventType != eventType {
			t.Fatalf("event_type = %q, want %q", notification.EventType, eventType)
		}
		if notification.LatestEventCategory != "mention" {
			t.Fatalf("latest_event_category = %q, want mention", notification.LatestEventCategory)
		}
		if notification.Priority != "high" {
			t.Fatalf("priority = %q, want high", notification.Priority)
		}
	}
	sort.Strings(gotRecipients)
	if len(gotRecipients) != len(wantRecipients) {
		t.Fatalf("recipient count = %d, want %d", len(gotRecipients), len(wantRecipients))
	}
	for i := range gotRecipients {
		if gotRecipients[i] != wantRecipients[i] {
			t.Fatalf("recipient[%d] = %q, want %q", i, gotRecipients[i], wantRecipients[i])
		}
	}
}

func TestResolveMentionRecipients_TeamMentionsUseReadableTeamMembers(t *testing.T) {
	t.Parallel()

	env := newPMMentionTestEnv(t)
	recipients, err := resolveMentionRecipients(context.Background(), env.workspaceRepo, env.workspaceID, pmMentionBodyHTML, env.actorUserID, []string{env.engTeamID})
	if err != nil {
		t.Fatalf("resolveMentionRecipients: %v", err)
	}

	want := env.expectedRecipientIDs()
	sort.Strings(recipients)
	if len(recipients) != len(want) {
		t.Fatalf("recipient count = %d, want %d (%v)", len(recipients), len(want), recipients)
	}
	for i := range recipients {
		if recipients[i] != want[i] {
			t.Fatalf("recipient[%d] = %q, want %q", i, recipients[i], want[i])
		}
	}
}

func TestResolveMentionRecipients_IgnoresOutOfScopeTeamHandles(t *testing.T) {
	t.Parallel()

	env := newPMMentionTestEnv(t)
	env.addCarolToDesignTeam(t)

	recipients, err := resolveMentionRecipients(context.Background(), env.workspaceRepo, env.workspaceID, "@design", env.actorUserID, []string{env.engTeamID})
	if err != nil {
		t.Fatalf("resolveMentionRecipients: %v", err)
	}
	if len(recipients) != 0 {
		t.Fatalf("recipient count = %d, want 0 (%v)", len(recipients), recipients)
	}
}

func TestPMTaskService_CreateAndUpdate_TeamMentions(t *testing.T) {
	t.Parallel()

	t.Run("create emits story mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		story := env.createStory(t, stringPtr(pmMentionBodyHTML))
		assertMentionNotifications(t, env.notificationsFor(t, "story", story.Story.ID), "story.mention", env.expectedRecipientIDs())
	})

	t.Run("update emits story mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		story := env.createStory(t, nil)
		updated, err := env.storyService.Update(context.Background(), story.Story.ID, model.UpdateTaskRequest{
			Description: stringPtr(pmMentionBodyHTML),
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("update story: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "story", updated.Story.ID), "story.mention", env.expectedRecipientIDs())
	})

	t.Run("create ignores out-of-scope team mentions", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		env.addCarolToDesignTeam(t)

		story := env.createStory(t, stringPtr("@design"))
		notifications := env.notificationsFor(t, "story", story.Story.ID)
		if len(notifications) != 0 {
			t.Fatalf("notification count = %d, want 0", len(notifications))
		}
	})
}

func TestPMCommentService_CreateAndUpdate_TeamMentions(t *testing.T) {
	t.Parallel()

	t.Run("create emits comment mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		story := env.createStory(t, nil)
		comment, err := env.commentService.Create(context.Background(), model.CreateCommentRequest{
			EntityType: "story",
			EntityID:   story.Story.ID,
			Body:       pmMentionBodyText,
		}, env.actorUserID, env.workspaceID)
		if err != nil {
			t.Fatalf("create comment: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "story", comment.Comment.EntityID), "comment.mention", env.expectedRecipientIDs())
	})

	t.Run("create keeps follower notifications separate from mentions", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		story := env.createStory(t, nil)
		mustExec(t, env.db, `INSERT INTO entity_followers (id, user_id, entity_type, entity_id, workspace_id, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"follower-carol-story", env.carolUserID, "story", story.Story.ID, env.workspaceID, "watching", time.Now().UTC())

		comment, err := env.commentService.Create(context.Background(), model.CreateCommentRequest{
			EntityType: "story",
			EntityID:   story.Story.ID,
			Body:       pmCommentMentionFollowerText,
		}, env.actorUserID, env.workspaceID)
		if err != nil {
			t.Fatalf("create comment: %v", err)
		}

		notifications := env.notificationsFor(t, "story", comment.Comment.EntityID)
		if len(notifications) != 3 {
			t.Fatalf("notification count = %d, want 3", len(notifications))
		}

		gotByRecipient := make(map[string]model.Notification, len(notifications))
		for _, notification := range notifications {
			gotByRecipient[notification.RecipientID] = notification
		}

		for _, recipientID := range []string{env.aliceUserID, env.bobUserID} {
			notification, ok := gotByRecipient[recipientID]
			if !ok {
				t.Fatalf("missing mention notification for %s", recipientID)
			}
			if notification.EventType != "comment.mention" {
				t.Fatalf("recipient %s event_type = %q, want comment.mention", recipientID, notification.EventType)
			}
			if notification.LatestEventCategory != "mention" {
				t.Fatalf("recipient %s latest_event_category = %q, want mention", recipientID, notification.LatestEventCategory)
			}
		}

		followerNotification, ok := gotByRecipient[env.carolUserID]
		if !ok {
			t.Fatalf("missing follower notification for %s", env.carolUserID)
		}
		if followerNotification.EventType != "comment.created" {
			t.Fatalf("follower event_type = %q, want comment.created", followerNotification.EventType)
		}
		if followerNotification.LatestEventCategory != "comment" {
			t.Fatalf("follower latest_event_category = %q, want comment", followerNotification.LatestEventCategory)
		}
	})

	t.Run("update emits comment mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		story := env.createStory(t, nil)
		comment, err := env.commentService.Create(context.Background(), model.CreateCommentRequest{
			EntityType: "story",
			EntityID:   story.Story.ID,
			Body:       "plain comment",
		}, env.actorUserID, env.workspaceID)
		if err != nil {
			t.Fatalf("create comment: %v", err)
		}
		if _, err := env.commentService.Update(context.Background(), comment.Comment.ID, model.UpdateCommentRequest{
			Body: pmMentionBodyText,
		}, env.actorUserID, true, env.workspaceID); err != nil {
			t.Fatalf("update comment: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "story", story.Story.ID), "comment.mention", env.expectedRecipientIDs())
	})

	t.Run("create ignores out-of-scope team mentions", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		env.addCarolToDesignTeam(t)
		story := env.createStory(t, nil)
		comment, err := env.commentService.Create(context.Background(), model.CreateCommentRequest{
			EntityType: "story",
			EntityID:   story.Story.ID,
			Body:       "@design",
		}, env.actorUserID, env.workspaceID)
		if err != nil {
			t.Fatalf("create comment: %v", err)
		}

		notifications := env.notificationsFor(t, "story", comment.Comment.EntityID)
		if len(notifications) != 0 {
			t.Fatalf("notification count = %d, want 0", len(notifications))
		}
	})
}

func TestPMChecklistItemService_CreateAndUpdate_TeamMentions(t *testing.T) {
	t.Parallel()

	t.Run("create emits checklist mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		story := env.createStory(t, nil)
		item, err := env.checklistService.Create(context.Background(), story.Story.ID, model.CreateChecklistItemRequest{
			Text: pmMentionBodyText,
		}, env.workspaceID, env.actorUserID)
		if err != nil {
			t.Fatalf("create checklist item: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "story", item.TaskID), "checklist.mention", env.expectedRecipientIDs())
	})

	t.Run("update emits checklist mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		story := env.createStory(t, nil)
		item, err := env.checklistService.Create(context.Background(), story.Story.ID, model.CreateChecklistItemRequest{
			Text: "plain item",
		}, env.workspaceID, env.actorUserID)
		if err != nil {
			t.Fatalf("create checklist item: %v", err)
		}
		if _, err := env.checklistService.Update(context.Background(), item.ID, model.UpdateChecklistItemRequest{
			Text: stringPtr(pmMentionBodyText),
		}, env.workspaceID, env.actorUserID); err != nil {
			t.Fatalf("update checklist item: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "story", story.Story.ID), "checklist.mention", env.expectedRecipientIDs())
	})
}

func TestPMPlanningEntityServices_TeamMentions(t *testing.T) {
	t.Parallel()

	t.Run("epic create and update emit mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		epic, err := env.epicService.Create(context.Background(), model.CreateEpicRequest{
			WorkspaceID: env.workspaceID,
			Name:        "Mention Epic",
			Description: stringPtr(pmMentionBodyHTML),
			TeamID:      stringPtr(env.engTeamID),
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("create epic: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "epic", epic.Epic.ID), "epic.mention", env.expectedRecipientIDs())

		env = newPMMentionTestEnv(t)
		epic, err = env.epicService.Create(context.Background(), model.CreateEpicRequest{
			WorkspaceID: env.workspaceID,
			Name:        "Plain Epic",
			TeamID:      stringPtr(env.engTeamID),
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("create plain epic: %v", err)
		}
		if _, err := env.epicService.Update(context.Background(), epic.Epic.ID, model.UpdateEpicRequest{
			Description: stringPtr(pmMentionBodyHTML),
		}, env.actorUserID); err != nil {
			t.Fatalf("update epic: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "epic", epic.Epic.ID), "epic.mention", env.expectedRecipientIDs())
	})

	t.Run("sprint create and update emit mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		start := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.April, 3, 0, 0, 0, 0, time.UTC)
		sprint, err := env.sprintService.Create(context.Background(), model.CreateSprintRequest{
			WorkspaceID: env.workspaceID,
			Name:        "Mention Sprint",
			Description: stringPtr(pmMentionBodyHTML),
			StartDate:   start,
			EndDate:     end,
			TeamID:      stringPtr(env.engTeamID),
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("create sprint: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "sprint", sprint.Sprint.ID), "sprint.mention", env.expectedRecipientIDs())

		env = newPMMentionTestEnv(t)
		sprint, err = env.sprintService.Create(context.Background(), model.CreateSprintRequest{
			WorkspaceID: env.workspaceID,
			Name:        "Plain Sprint",
			StartDate:   start,
			EndDate:     end,
			TeamID:      stringPtr(env.engTeamID),
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("create plain sprint: %v", err)
		}
		if _, err := env.sprintService.Update(context.Background(), sprint.Sprint.ID, model.UpdateSprintRequest{
			Description: stringPtr(pmMentionBodyHTML),
		}, env.actorUserID); err != nil {
			t.Fatalf("update sprint: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "sprint", sprint.Sprint.ID), "sprint.mention", env.expectedRecipientIDs())
	})

	t.Run("objective create and update emit mention notifications", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		objective, err := env.objectiveService.Create(context.Background(), model.CreateObjectiveRequest{
			WorkspaceID: env.workspaceID,
			Name:        "Mention Objective",
			Description: stringPtr(pmMentionBodyHTML),
			TeamIDs:     []string{env.engTeamID},
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("create objective: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "objective", objective.Objective.ID), "objective.mention", env.expectedRecipientIDs())

		env = newPMMentionTestEnv(t)
		objective, err = env.objectiveService.Create(context.Background(), model.CreateObjectiveRequest{
			WorkspaceID: env.workspaceID,
			Name:        "Plain Objective",
			TeamIDs:     []string{env.engTeamID},
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("create plain objective: %v", err)
		}
		if _, err := env.objectiveService.Update(context.Background(), objective.Objective.ID, model.UpdateObjectiveRequest{
			Description: stringPtr(pmMentionBodyHTML),
		}, env.actorUserID); err != nil {
			t.Fatalf("update objective: %v", err)
		}
		assertMentionNotifications(t, env.notificationsFor(t, "objective", objective.Objective.ID), "objective.mention", env.expectedRecipientIDs())
	})

	t.Run("objective ignores out-of-scope team mentions", func(t *testing.T) {
		env := newPMMentionTestEnv(t)
		env.addCarolToDesignTeam(t)

		objective, err := env.objectiveService.Create(context.Background(), model.CreateObjectiveRequest{
			WorkspaceID: env.workspaceID,
			Name:        "Design Ping Objective",
			Description: stringPtr("@design"),
			TeamIDs:     []string{env.engTeamID},
		}, env.actorUserID)
		if err != nil {
			t.Fatalf("create objective: %v", err)
		}

		notifications := env.notificationsFor(t, "objective", objective.Objective.ID)
		if len(notifications) != 0 {
			t.Fatalf("notification count = %d, want 0", len(notifications))
		}
	})
}
