package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestRuntimeAttentionNotificationLifecycle(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	seedNotificationServiceUser(t, db, "owner", "owner@example.com", "Owner", time.Now())
	notifications := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, repository.NewUserRepository(db), nil, nil, nil, "")
	repo := &fakeAgentRuntimeProjectionInteractionRepo{}
	projection := (&AgentRuntimeProjectionService{interactionRepo: repo, runRepo: &fakeAgentRuntimeProjectionRunRepo{}}).SetAttentionNotifier(notifications)
	run := &model.AgentRun{ID: "run", WorkspaceID: "ws", DockChatID: strPtr("chat"), TriggeredByUserID: strPtr("owner"), Status: model.AgentRunStatusPaused, RuntimeKind: "native_sdk"}
	interaction := AgentRuntimeInteraction{ID: "question", InteractionKind: model.AgentRunInteractionKindRequestUserInput, Status: model.AgentRunInteractionStatusPending, Title: "Which project?"}
	project := func() {
		t.Helper()
		if err := projection.upsertRuntimeInteraction(ctx, run, interaction); err != nil {
			t.Fatal(err)
		}
	}
	check := func(status string, count int) {
		t.Helper()
		var notification model.Notification
		if err := db.First(&notification).Error; err != nil {
			t.Fatal(err)
		}
		if notification.Status != status || notification.EventCount != count || notification.RecipientID != "owner" || notification.Metadata["dock_chat_id"] != "chat" {
			t.Fatalf("unexpected notification: %+v", notification)
		}
		assertNotificationServiceCount(t, db, "notification_events", int64(count))
	}
	project()
	project() // Repeated snapshot.
	check("unread", 1)
	if err := notifications.MarkAgentAttentionResolved(ctx, "ws", "run"); err != nil {
		t.Fatal(err)
	}
	project() // A read alert must not be resurrected.
	check("read", 1)
	interaction.ID = "approval"
	interaction.InteractionKind = model.AgentRunInteractionKindApprovalRequest
	project()
	check("unread", 2)
	interaction.ID = "question"
	interaction.InteractionKind = model.AgentRunInteractionKindRequestUserInput
	project() // Replaying an older simultaneous interaction must also deduplicate.
	check("unread", 2)
	// The fake repository needs distinct database IDs for update matching.
	for i := range repo.interactions {
		repo.interactions[i].ID = derefString(repo.interactions[i].RequestID)
	}
	interaction.Status = model.AgentRunInteractionStatusResolved
	project() // The second interaction still needs attention.
	check("unread", 2)
	interaction.ID = "approval"
	interaction.InteractionKind = model.AgentRunInteractionKindApprovalRequest
	project()
	check("read", 2)
	interaction.ID = "last"
	interaction.Status = model.AgentRunInteractionStatusPending
	project()
	check("unread", 3)
	run.Status = model.AgentRunStatusCancelled
	project() // Terminal runs clear even if runtime still reports a pending item.
	check("read", 3)
	for _, delivery := range loadNotificationServiceDeliveries(t, db) {
		if delivery.Channel != "in_app" && delivery.Status != "skipped" {
			t.Fatalf("unexpected email delivery: %+v", delivery)
		}
	}
}

type attentionRecorder struct {
	events   []model.NotificationEventInput
	resolved []string
}

func (n *attentionRecorder) Emit(_ context.Context, event model.NotificationEventInput) error {
	n.events = append(n.events, event)
	return nil
}
func (n *attentionRecorder) MarkAgentAttentionResolved(_ context.Context, _, runID string) error {
	n.resolved = append(n.resolved, runID)
	return nil
}

func TestRuntimeTerminalProjectionClearsAttentionWithoutInteractions(t *testing.T) {
	for _, eventType := range []string{agentruntime.EventRunCompleted, agentruntime.EventRunCancelled, agentruntime.EventRunFailed} {
		t.Run(eventType, func(t *testing.T) {
			run := &model.AgentRun{ID: "run", WorkspaceID: "ws", Status: model.AgentRunStatusRunning, ExternalRuntime: strPtr(agentRuntimeName), ExternalRuntimeID: strPtr("runtime-run")}
			n := &attentionRecorder{}
			s := (&AgentRuntimeProjectionService{
				runRepo: &fakeAgentRuntimeProjectionRunRepo{byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime-run": run}},
				now:     time.Now,
			}).SetAttentionNotifier(n)
			if err := s.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{RunID: "runtime-run", Type: eventType, SentAt: time.Now(), Data: map[string]any{}}); err != nil {
				t.Fatal(err)
			}
			if len(n.resolved) != 1 || n.resolved[0] != "run" {
				t.Fatalf("resolved: %v", n.resolved)
			}
		})
	}
}

func TestRuntimeAttentionKindsAndRouting(t *testing.T) {
	for _, kind := range []string{model.AgentRunInteractionKindRequestUserInput, model.AgentRunInteractionKindApprovalRequest, model.AgentRunInteractionKindReviewCheckpoint, model.AgentRunInteractionKindCommandExecutionApproval, model.AgentRunInteractionKindFileChangeApproval, model.AgentRunInteractionKindPermissionsApproval} {
		t.Run(kind, func(t *testing.T) {
			n := &attentionRecorder{}
			s := (&AgentRuntimeProjectionService{}).SetAttentionNotifier(n)
			run := &model.AgentRun{ID: "run", WorkspaceID: "ws", TargetType: "task", TargetID: "task", TriggeredByUserID: strPtr("owner"), Status: model.AgentRunStatusRunning}
			interaction := model.AgentRunInteraction{ID: "interaction", Status: model.AgentRunInteractionStatusPending, InteractionKind: kind}
			if err := s.syncInteractionAttention(context.Background(), run, interaction); err != nil {
				t.Fatal(err)
			}
			if len(n.events) != 1 {
				t.Fatalf("events = %d", len(n.events))
			}
			e := n.events[0]
			if len(e.ExplicitRecipients) != 1 || e.ExplicitRecipients[0] != "owner" || !e.SkipFollowers || e.SkipEmailDelivery || e.Metadata["task_id"] != "task" {
				t.Fatalf("unexpected routing: %+v", e)
			}
			var payload notificationWSData
			if err := json.Unmarshal(buildNotificationWSData("owner", e, "high", "unread"), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.RunID != "run" || payload.ParentTaskID != "task" {
				t.Fatalf("payload: %+v", payload)
			}
			run.TriggeredByUserID = nil
			if err := s.syncInteractionAttention(context.Background(), run, interaction); err != nil {
				t.Fatal(err)
			}
			if len(n.events) != 1 {
				t.Fatal("notified without an owner")
			}
		})
	}
}

func TestWorkflowFailureAlertsOwnerOncePerFailedRun(t *testing.T) {
	ctx := context.Background()
	rules := repository.NewAutomationRuleRepository(setupRuleEngineTestDB(t))
	rule := &model.AutomationRule{
		ID: "rule", WorkspaceID: "ws", Name: "Daily triage", CreatedBy: strPtr("workflow-owner"),
		TriggerType: model.TriggerCron, TriggerConfig: json.RawMessage(`{}`),
		ActionType: model.ActionStartAgentRun, ActionConfig: json.RawMessage(`{}`),
	}
	if err := rules.Create(ctx, rule); err != nil {
		t.Fatal(err)
	}
	db := newNotificationServiceTestDB(t)
	projection := (&AgentRuntimeProjectionService{}).SetNotificationRules(rules).SetAttentionNotifier(newCRUDNotificationService(db))
	run := &model.AgentRun{
		WorkspaceID: "ws", Status: model.AgentRunStatusFailed, TriggeredByUserID: strPtr("requester"),
		Input: json.RawMessage(`{"trigger":{"rule_id":"rule"}}`),
	}
	for _, id := range []string{"run", "run", "another", "run"} {
		run.ID = id
		if err := projection.notifyRunActivity(ctx, run, "agent_run.finished"); err != nil {
			t.Fatal(err)
		}
	}
	var items []model.Notification
	if err := db.Find(&items).Error; err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RecipientID != "workflow-owner" || items[0].EventCount != 2 || items[0].EntityID != "rule" || items[0].EventType != "automation_rule.failed" {
		t.Fatalf("workflow failures: %+v", items)
	}
	if err := db.Exec("INSERT INTO notification_preferences(user_id,workspace_id,mute_workspace,channel_preferences) VALUES ('workflow-owner','ws',false,?)", []byte(`{"automation.failures":{"in_app":false,"email":false},"automation.activity":{"in_app":true}}`)).Error; err != nil {
		t.Fatal(err)
	}
	run.ID = "silenced"
	if err := projection.notifyRunActivity(ctx, run, "agent_run.finished"); err != nil {
		t.Fatal(err)
	}
	assertNotificationServiceCount(t, db, "notification_events", 2)
}
