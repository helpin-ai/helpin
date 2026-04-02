package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type recurringTestEnv struct {
	storyEnv storyTestEnv
	svc      *PMRecurringTemplateService
	repo     *repository.PMRecurringTemplateRepository
}

func newRecurringTestEnv(t *testing.T) recurringTestEnv {
	t.Helper()
	storyEnv := newStoryTestEnv(t)
	recurringRepo := repository.NewPMRecurringTemplateRepository(storyEnv.db)
	recurringSvc := NewPMRecurringTemplateService(
		recurringRepo,
		repository.NewPMStoryRepository(storyEnv.db),
		repository.NewPMWorkflowRepository(storyEnv.db),
		repository.NewPMSprintRepository(storyEnv.db),
		repository.NewWorkspaceRepository(storyEnv.db),
		repository.NewPMChecklistItemRepository(storyEnv.db),
		repository.NewPMExternalLinkRepository(storyEnv.db),
		NewPMActivityService(repository.NewPMActivityRepository(storyEnv.db)),
		nil,
	)
	recurringSvc.SetStoryService(storyEnv.svc)
	storyEnv.svc.SetRecurringService(recurringSvc)
	return recurringTestEnv{
		storyEnv: storyEnv,
		svc:      recurringSvc,
		repo:     recurringRepo,
	}
}

func TestPMRecurringTemplateService_CreateFromStory(t *testing.T) {
	t.Parallel()
	env := newRecurringTestEnv(t)
	ctx := context.Background()

	story := createTestStory(t, env.storyEnv, "Weekly Ops Check")
	cfg := model.PMRecurringTemplateConfig{
		ScheduleType: model.PMRecurringScheduleTypeTime,
		Frequency:    model.PMRecurringFrequencyWeekly,
		Interval:     1,
		Weekdays:     []int{1, 3},
		DueDateMode:  model.PMRecurringDueDateModeScheduled,
	}

	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.storyEnv.wsID,
		TaskID:     story.Story.ID,
		Title:       "Weekly Ops Check",
		Config:      cfg,
	}, env.storyEnv.userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tmpl.Template.GeneratedCount != 1 {
		t.Fatalf("generated_count = %d, want 1", tmpl.Template.GeneratedCount)
	}
	if tmpl.Template.LastGeneratedTaskID == nil || *tmpl.Template.LastGeneratedTaskID != story.Story.ID {
		t.Fatalf("last_generated_story_id = %v, want %q", tmpl.Template.LastGeneratedTaskID, story.Story.ID)
	}
	if tmpl.Template.NextRunAt == nil {
		t.Fatal("expected next_run_at to be populated for time-based template")
	}

	rawStory, err := env.storyEnv.svc.storyRepo.GetRawByID(ctx, story.Story.ID)
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

	story := createTestStory(t, env.storyEnv, "Daily Standup")
	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.storyEnv.wsID,
		TaskID:     story.Story.ID,
		Title:       "Daily Standup",
		Config: model.PMRecurringTemplateConfig{
			ScheduleType: model.PMRecurringScheduleTypeTime,
			Frequency:    model.PMRecurringFrequencyDaily,
			Interval:     1,
			DueDateMode:  model.PMRecurringDueDateModeScheduled,
		},
	}, env.storyEnv.userID)
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
	if updatedTemplate.LastGeneratedTaskID == nil || *updatedTemplate.LastGeneratedTaskID == story.Story.ID {
		t.Fatalf("expected a new generated story, got %v", updatedTemplate.LastGeneratedTaskID)
	}

	generatedStory, err := env.storyEnv.svc.storyRepo.GetRawByID(ctx, *updatedTemplate.LastGeneratedTaskID)
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

	story := createTestStory(t, env.storyEnv, "Post-deploy Checklist")
	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.storyEnv.wsID,
		TaskID:     story.Story.ID,
		Title:       "Post-deploy Checklist",
		Config: model.PMRecurringTemplateConfig{
			ScheduleType:    model.PMRecurringScheduleTypeCompletion,
			CompletionEvent: model.PMRecurringCompletionEventDoneState,
			DueDateMode:     model.PMRecurringDueDateModeOffsetDays,
			DueOffsetDays:   recurringIntPtr(2),
		},
	}, env.storyEnv.userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := env.storyEnv.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
		WorkflowStateID: &env.storyEnv.stDone,
	}, env.storyEnv.userID); err != nil {
		t.Fatalf("Update story to done: %v", err)
	}

	updatedTemplate, err := env.repo.GetByID(ctx, tmpl.Template.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updatedTemplate.GeneratedCount != 2 {
		t.Fatalf("generated_count = %d, want 2", updatedTemplate.GeneratedCount)
	}
	if updatedTemplate.LastGeneratedTaskID == nil || *updatedTemplate.LastGeneratedTaskID == story.Story.ID {
		t.Fatalf("expected a new generated story, got %v", updatedTemplate.LastGeneratedTaskID)
	}
	generatedStory, err := env.storyEnv.svc.storyRepo.GetRawByID(ctx, *updatedTemplate.LastGeneratedTaskID)
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

	story := createTestStory(t, env.storyEnv, "Monthly Audit")
	tmpl, err := env.svc.Create(ctx, model.CreateRecurringTemplateRequest{
		WorkspaceID: env.storyEnv.wsID,
		TaskID:     story.Story.ID,
		Title:       "Monthly Audit",
		Config: model.PMRecurringTemplateConfig{
			ScheduleType: model.PMRecurringScheduleTypeTime,
			Frequency:    model.PMRecurringFrequencyMonthly,
			Interval:     1,
			DueDateMode:  model.PMRecurringDueDateModeScheduled,
		},
	}, env.storyEnv.userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updatedName := "Monthly Compliance Audit"
	updatedPriority := model.PMStoryPriorityUrgent
	if _, err := env.storyEnv.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
		Name:     &updatedName,
		Priority: &updatedPriority,
	}, env.storyEnv.userID); err != nil {
		t.Fatalf("Update story: %v", err)
	}

	if _, err := env.svc.Update(ctx, tmpl.Template.ID, model.UpdateRecurringTemplateRequest{
		TaskID: &story.Story.ID,
	}, env.storyEnv.userID); err != nil {
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

	generatedStory, err := env.storyEnv.svc.storyRepo.GetRawByID(ctx, *updatedTemplate.LastGeneratedTaskID)
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
