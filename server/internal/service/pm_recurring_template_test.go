package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type recurringTestEnv struct {
	taskEnv taskTestEnv
	svc      *PMRecurringTemplateService
	repo     *repository.PMRecurringTemplateRepository
}

func newRecurringTestEnv(t *testing.T) recurringTestEnv {
	t.Helper()
	taskEnv := newTaskTestEnv(t)
	recurringRepo := repository.NewPMRecurringTemplateRepository(taskEnv.db)
	recurringSvc := NewPMRecurringTemplateService(
		recurringRepo,
		repository.NewPMTaskRepository(taskEnv.db),
		repository.NewPMWorkflowRepository(taskEnv.db),
		repository.NewPMSprintRepository(taskEnv.db),
		repository.NewWorkspaceRepository(taskEnv.db),
		repository.NewPMChecklistItemRepository(taskEnv.db),
		repository.NewPMExternalLinkRepository(taskEnv.db),
		NewPMActivityService(repository.NewPMActivityRepository(taskEnv.db)),
		nil,
	)
	recurringSvc.SetTaskService(taskEnv.svc)
	taskEnv.svc.SetRecurringService(recurringSvc)
	return recurringTestEnv{
		taskEnv: taskEnv,
		svc:      recurringSvc,
		repo:     recurringRepo,
	}
}

func TestPMRecurringTemplateService_CreateFromStory(t *testing.T) {
	t.Parallel()
	env := newRecurringTestEnv(t)
	ctx := context.Background()

	story := createTestTask(t, env.taskEnv, "Weekly Ops Check")
	cfg := model.PMRecurringTemplateConfig{
		ScheduleType: model.PMRecurringScheduleTypeTime,
		Frequency:    model.PMRecurringFrequencyWeekly,
		Interval:     1,
		Weekdays:     []int{1, 3},
		DueDateMode:  model.PMRecurringDueDateModeScheduled,
	}

	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.taskEnv.wsID,
		TaskID:     story.Task.ID,
		Title:       "Weekly Ops Check",
		Config:      cfg,
	}, env.taskEnv.userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tmpl.Template.GeneratedCount != 1 {
		t.Fatalf("generated_count = %d, want 1", tmpl.Template.GeneratedCount)
	}
	if tmpl.Template.LastGeneratedTaskID == nil || *tmpl.Template.LastGeneratedTaskID != story.Task.ID {
		t.Fatalf("last_generated_story_id = %v, want %q", tmpl.Template.LastGeneratedTaskID, story.Task.ID)
	}
	if tmpl.Template.NextRunAt == nil {
		t.Fatal("expected next_run_at to be populated for time-based template")
	}

	rawStory, err := env.taskEnv.svc.taskRepo.GetRawByID(ctx, story.Task.ID)
	if err != nil {
		t.Fatalf("GetRawByID: %v", err)
	}
	if rawStory.RecurringTemplateID == nil || *rawStory.RecurringTemplateID != tmpl.Template.ID {
		t.Fatalf("recurring_template_id = %v, want %q", rawStory.RecurringTemplateID, tmpl.Template.ID)
	}
	if rawStory.RecurringOccurrenceNumber == nil || *rawStory.RecurringOccurrenceNumber != 1 {
		t.Fatalf("recurring_occurrence_number = %v, want 1", rawStory.RecurringOccurrenceNumber)
	}
}

func TestPMRecurringTemplateService_ProcessDueTemplates(t *testing.T) {
	t.Parallel()
	env := newRecurringTestEnv(t)
	ctx := context.Background()

	story := createTestTask(t, env.taskEnv, "Daily Standup")
	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.taskEnv.wsID,
		TaskID:     story.Task.ID,
		Title:       "Daily Standup",
		Config: model.PMRecurringTemplateConfig{
			ScheduleType: model.PMRecurringScheduleTypeTime,
			Frequency:    model.PMRecurringFrequencyDaily,
			Interval:     1,
			DueDateMode:  model.PMRecurringDueDateModeScheduled,
		},
	}, env.taskEnv.userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rawTemplate, err := env.repo.GetByID(ctx, tmpl.Template.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	past := time.Now().UTC().Add(-24 * time.Hour)
	rawTemplate.NextRunAt = &past
	if err := env.repo.Update(ctx, rawTemplate); err != nil {
		t.Fatalf("Update: %v", err)
	}

	processed, err := env.svc.ProcessDueTemplates(ctx, 10)
	if err != nil {
		t.Fatalf("ProcessDueTemplates: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	updatedTemplate, err := env.repo.GetByID(ctx, tmpl.Template.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updatedTemplate.GeneratedCount != 2 {
		t.Fatalf("generated_count = %d, want 2", updatedTemplate.GeneratedCount)
	}
	if updatedTemplate.LastGeneratedTaskID == nil || *updatedTemplate.LastGeneratedTaskID == story.Task.ID {
		t.Fatalf("expected a new generated story, got %v", updatedTemplate.LastGeneratedTaskID)
	}

	generatedStory, err := env.taskEnv.svc.taskRepo.GetRawByID(ctx, *updatedTemplate.LastGeneratedTaskID)
	if err != nil {
		t.Fatalf("GetRawByID: %v", err)
	}
	if generatedStory.RecurringOccurrenceNumber == nil || *generatedStory.RecurringOccurrenceNumber != 2 {
		t.Fatalf("recurring_occurrence_number = %v, want 2", generatedStory.RecurringOccurrenceNumber)
	}
}

func TestPMRecurringTemplateService_CompletionBasedGeneration(t *testing.T) {
	t.Parallel()
	env := newRecurringTestEnv(t)
	ctx := context.Background()

	story := createTestTask(t, env.taskEnv, "Post-deploy Checklist")
	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.taskEnv.wsID,
		TaskID:     story.Task.ID,
		Title:       "Post-deploy Checklist",
		Config: model.PMRecurringTemplateConfig{
			ScheduleType:    model.PMRecurringScheduleTypeCompletion,
			CompletionEvent: model.PMRecurringCompletionEventDoneState,
			DueDateMode:     model.PMRecurringDueDateModeOffsetDays,
			DueOffsetDays:   recurringIntPtr(2),
		},
	}, env.taskEnv.userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := env.taskEnv.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
		WorkflowStateID: &env.taskEnv.stDone,
	}, env.taskEnv.userID); err != nil {
		t.Fatalf("Update story to done: %v", err)
	}

	updatedTemplate, err := env.repo.GetByID(ctx, tmpl.Template.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updatedTemplate.GeneratedCount != 2 {
		t.Fatalf("generated_count = %d, want 2", updatedTemplate.GeneratedCount)
	}
	if updatedTemplate.LastGeneratedTaskID == nil || *updatedTemplate.LastGeneratedTaskID == story.Task.ID {
		t.Fatalf("expected a new generated story, got %v", updatedTemplate.LastGeneratedTaskID)
	}
	generatedStory, err := env.taskEnv.svc.taskRepo.GetRawByID(ctx, *updatedTemplate.LastGeneratedTaskID)
	if err != nil {
		t.Fatalf("GetRawByID: %v", err)
	}
	if generatedStory.Deadline == nil {
		t.Fatal("expected completion-generated story to have a due date")
	}
	if generatedStory.RecurringOccurrenceNumber == nil || *generatedStory.RecurringOccurrenceNumber != 2 {
		t.Fatalf("recurring_occurrence_number = %v, want 2", generatedStory.RecurringOccurrenceNumber)
	}
}

func TestPMRecurringTemplateService_UpdateRefreshesSeedFromStory(t *testing.T) {
	t.Parallel()
	env := newRecurringTestEnv(t)
	ctx := context.Background()

	story := createTestTask(t, env.taskEnv, "Monthly Audit")
	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.taskEnv.wsID,
		TaskID:     story.Task.ID,
		Title:       "Monthly Audit",
		Config: model.PMRecurringTemplateConfig{
			ScheduleType: model.PMRecurringScheduleTypeTime,
			Frequency:    model.PMRecurringFrequencyMonthly,
			Interval:     1,
			DueDateMode:  model.PMRecurringDueDateModeScheduled,
		},
	}, env.taskEnv.userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updatedName := "Monthly Compliance Audit"
	updatedPriority := model.PMTaskPriorityUrgent
	if _, err := env.taskEnv.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
		Name:     &updatedName,
		Priority: &updatedPriority,
	}, env.taskEnv.userID); err != nil {
		t.Fatalf("Update story: %v", err)
	}

	if _, err := env.svc.Update(ctx, tmpl.Template.ID, model.UpdateRecurringTemplateRequest{
		TaskID: &story.Task.ID,
	}, env.taskEnv.userID); err != nil {
		t.Fatalf("Update recurring template: %v", err)
	}

	rawTemplate, err := env.repo.GetByID(ctx, tmpl.Template.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	past := time.Now().UTC().Add(-24 * time.Hour)
	rawTemplate.NextRunAt = &past
	if err := env.repo.Update(ctx, rawTemplate); err != nil {
		t.Fatalf("Update: %v", err)
	}

	processed, err := env.svc.ProcessDueTemplates(ctx, 10)
	if err != nil {
		t.Fatalf("ProcessDueTemplates: %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	updatedTemplate, err := env.repo.GetByID(ctx, tmpl.Template.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updatedTemplate.LastGeneratedTaskID == nil {
		t.Fatal("expected a generated story after refresh")
	}

	generatedStory, err := env.taskEnv.svc.taskRepo.GetRawByID(ctx, *updatedTemplate.LastGeneratedTaskID)
	if err != nil {
		t.Fatalf("GetRawByID: %v", err)
	}
	if generatedStory.Name != updatedName {
		t.Fatalf("generated story name = %q, want %q", generatedStory.Name, updatedName)
	}
	if generatedStory.Priority != updatedPriority {
		t.Fatalf("generated story priority = %q, want %q", generatedStory.Priority, updatedPriority)
	}
}
