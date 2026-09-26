package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPMTriageReviewDismissalPersists(t *testing.T) {
	triage, provider, _, _ := setupPMTriageService(t)
	view, err := triage.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
	if err != nil {
		t.Fatal(err)
	}
	reviewer := NewPMTriageReviewService(triage, nil, nil)
	req := model.PMTriageReviewRequest{AssessmentID: view.ID, Action: "match", Value: "match", Dismiss: true}
	result, err := reviewer.Review(pmTriageMemberContext(), "workspace", "task", "source", req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Key != "match:match" || result.Status != "dismissed" {
		t.Fatalf("unexpected result %+v", result)
	}
	refreshed, err := triage.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Reviewed["match:match"] != "dismissed" || provider.calls != 1 {
		t.Fatal("dismissal lost or repeated provider call")
	}
	if _, err := reviewer.Review(pmTriageMemberContext(), "workspace", "task", "source", req); err != nil {
		t.Fatal("repeated dismissal was not idempotent", err)
	}
}
func TestPMTriageReviewRejectsForgedOrStaleActions(t *testing.T) {
	cases := []string{"different actor", "invented choice", "changed source", "changed target", "target moved team", "disabled"}
	for _, scenario := range cases {
		t.Run(scenario, func(t *testing.T) {
			triage, _, _, db := setupPMTriageService(t)
			ctx := pmTriageMemberContext()
			view, err := triage.Analyze(ctx, "workspace", "task", "source")
			if err != nil {
				t.Fatal(err)
			}
			req := model.PMTriageReviewRequest{AssessmentID: view.ID, Action: "match", Value: "match"}
			switch scenario {
			case "different actor":
				ctx = authorization.WithActor(context.Background(), &authorization.Actor{UserID: "other", WorkspaceID: "workspace", Role: model.RoleMember, TeamMemberships: []authorization.TeamRole{{TeamID: "mine"}}})
			case "invented choice":
				req.Value = "secret"
			case "changed source":
				if err := db.Exec("UPDATE pm_tasks SET name = ? WHERE id = ?", "New work", "source").Error; err != nil {
					t.Fatal(err)
				}
			case "changed target":
				if err := db.Exec("UPDATE pm_tasks SET description = ? WHERE id = ?", "Different issue", "match").Error; err != nil {
					t.Fatal(err)
				}
			case "target moved team":
				if err := db.Exec("UPDATE pm_tasks SET team_id = ? WHERE id = ?", "other", "match").Error; err != nil {
					t.Fatal(err)
				}
			case "disabled":
				triage.config.Mode = "off"
			}
			_, err = NewPMTriageReviewService(triage, nil, nil).Review(ctx, "workspace", "task", "source", req)
			if !errors.Is(err, ErrPMTriageStale) {
				t.Fatalf("expected stale refusal, got %v", err)
			}
		})
	}
}
func TestPMTriageReviewedRelationship(t *testing.T) {
	triage, _, _, db := setupPMTriageService(t)
	if err := db.Exec(`CREATE TABLE pm_task_links (id TEXT PRIMARY KEY,workspace_id TEXT,source_task_id TEXT,target_task_id TEXT,link_type TEXT,created_by TEXT,created_at DATETIME,updated_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	associations := &AssociationsService{taskRepo: triage.tasks, taskLinkRepo: repository.NewPMTaskLinkRepository(db)}
	view, err := triage.Analyze(pmTriageMemberContext(), "workspace", "task", "source")
	if err != nil {
		t.Fatal(err)
	}
	reviewer := NewPMTriageReviewService(triage, nil, associations)
	req := model.PMTriageReviewRequest{AssessmentID: view.ID, Action: "match", Value: "match"}
	for i := 0; i < 2; i++ {
		if _, err := reviewer.Review(pmTriageMemberContext(), "workspace", "task", "source", req); err != nil {
			t.Fatal(err)
		}
	}
	var links []model.PMTaskLink
	if err := db.Find(&links).Error; err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].SourceTaskID != "source" || links[0].TargetTaskID != "match" || links[0].LinkType != "duplicates" {
		t.Fatalf("unexpected relationships %+v", links)
	}
}
func TestPMTriageTaskRevisionFence(t *testing.T) {
	triage, _, _, _ := setupPMTriageService(t)
	err := triage.tasks.WithMutationTransaction(pmTriageMemberContext(), func(tasks *repository.PMTaskRepository, _ *repository.PMChecklistItemRepository) error {
		return tasks.RequireRevision(pmTriageMemberContext(), "source", time.Now())
	})
	if !errors.Is(err, repository.ErrPMTaskRevisionChanged) {
		t.Fatalf("expected stale refusal, got %v", err)
	}
}
func TestPMTriageSupportEvidenceExcludesInternalContent(t *testing.T) {
	conversation := &model.SupportConversation{Subject: "CSV export fails"}
	public := model.SupportMessage{ID: "public", SenderType: "customer", MessageType: "reply", Content: "Invoice export crashes"}
	messages := []model.SupportMessage{public, {ID: "private", IsInternal: true, Content: "SECRET INTERNAL"}, {ID: "system", MessageType: "system", Content: "SECRET SYSTEM"}}
	text, hash := supportPMTriageEvidence(conversation, messages)
	expected, publicHash := supportPMTriageEvidence(conversation, []model.SupportMessage{public})
	if text != expected || hash != publicHash {
		t.Fatal("internal or system content entered source evidence")
	}
	messages[0].Content = "Different customer evidence"
	_, changed := supportPMTriageEvidence(conversation, messages)
	if changed == hash {
		t.Fatal("public edit did not invalidate source fingerprint")
	}
}

func TestPMTriageReviewUsesCanonicalTaskUpdates(t *testing.T) {
	for _, action := range []string{"task_type", "team"} {
		t.Run(action, func(t *testing.T) {
			env := newTaskTestEnv(t)
			if err := env.db.AutoMigrate(&model.PMTriageAssessment{}); err != nil {
				t.Fatal(err)
			}
			ctx := authorization.WithActor(context.Background(), &authorization.Actor{UserID: env.userID, WorkspaceID: env.wsID, Role: model.RoleAdmin})
			priority := "high"
			created, err := env.svc.Create(ctx, model.CreateTaskRequest{WorkspaceID: env.wsID, Name: "CSV export fails", TaskType: "feature", Priority: &priority, TeamID: &env.teamID, WorkflowID: env.wfID, WorkflowStateID: env.stTodo}, env.userID)
			if err != nil {
				t.Fatal(err)
			}
			triage, err := NewPMTriageService(PMTriageConfig{Mode: "primary", Threshold: .95, DailyLimit: 10}, &pmTriageTestProvider{}, repository.NewPMTriageRepository(env.db), &pmTriageTestUsage{}, env.svc.taskRepo, env.svc.workspaceRepo, env.svc.labelRepo, &SupportInboxService{})
			if err != nil {
				t.Fatal(err)
			}
			source, err := triage.loadSource(ctx, env.wsID, "task", created.Task.ID)
			if err != nil {
				t.Fatal(err)
			}
			value := "bug"
			if action == "team" {
				value = "review-team"
				seedTaskTeam(t, env, value, "Payments")
			}
			admission, err := triage.assessments.Reserve(ctx, model.PMTriageAssessment{WorkspaceID: env.wsID, ActorID: env.userID, SourceKind: "task", SourceID: created.Task.ID, SourceHash: source.hash, ContextHash: "test-review", Mode: "primary"}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if err := triage.assessments.Finish(ctx, env.wsID, env.userID, admission.Assessment.ID, "ready", model.JSONB{"assessment": map[string]any{"actionable": true, action: map[string]any{"id": value, "probability": .99}}}); err != nil {
				t.Fatal(err)
			}
			_, err = NewPMTriageReviewService(triage, env.svc, nil).Review(ctx, env.wsID, "task", created.Task.ID, model.PMTriageReviewRequest{AssessmentID: admission.Assessment.ID, Action: action, Value: value})
			if err != nil {
				t.Fatal(err)
			}
			updated, err := env.svc.taskRepo.GetRawByID(ctx, created.Task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Priority != "high" {
				t.Fatal("classification changed priority")
			}
			if action == "task_type" && updated.TaskType != value {
				t.Fatal("type was not updated")
			}
			if action == "team" && derefString(updated.TeamID) != value {
				t.Fatal("team was not updated")
			}
		})
	}
}
