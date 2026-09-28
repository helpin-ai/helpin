package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestNormalizeObjectiveState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "canonical not_started", input: "not_started", want: model.PMObjectiveStateNotStarted, ok: true},
		{name: "legacy to_do", input: "to_do", want: model.PMObjectiveStateNotStarted, ok: true},
		{name: "label to do", input: "To Do", want: model.PMObjectiveStateNotStarted, ok: true},
		{name: "canonical active", input: "active", want: model.PMObjectiveStateActive, ok: true},
		{name: "legacy in_progress", input: "in_progress", want: model.PMObjectiveStateActive, ok: true},
		{name: "label in progress", input: "In Progress", want: model.PMObjectiveStateActive, ok: true},
		{name: "canonical closed", input: "closed", want: model.PMObjectiveStateClosed, ok: true},
		{name: "legacy done", input: "done", want: model.PMObjectiveStateClosed, ok: true},
		{name: "label completed", input: "Completed", want: model.PMObjectiveStateClosed, ok: true},
		{name: "invalid", input: "paused", want: "", ok: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := normalizeObjectiveState(tt.input)
			if ok != tt.ok {
				t.Fatalf("normalizeObjectiveState(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("normalizeObjectiveState(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPMObjectiveServiceUpdateCanClearDatesWithPresenceFlags(t *testing.T) {
	db := newTestDB(t)
	workspaceID := "ws-objective-date-clear"
	seedUser(t, db, "objective-date-admin", "objective-date@example.com", "Objective Date Admin", "hash")
	seedWorkspace(t, db, workspaceID, "Objective Date", "objective-date", "objective-date-admin")
	seedWorkspaceMember(t, db, "objective-date-member", workspaceID, "objective-date-admin", "objective-date@example.com", "Objective Date Admin", model.RoleAdmin)
	activity := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := NewPMObjectiveService(repository.NewPMObjectiveRepository(db), repository.NewPMKeyResultRepository(db), repository.NewPMLabelRepository(db), nil, repository.NewWorkspaceRepository(db), activity, nil, nil)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	created, err := svc.Create(context.Background(), model.CreateObjectiveRequest{WorkspaceID: workspaceID, Name: "Clear dates", ObjectiveType: model.PMObjectiveTypeStrategic, PlannedStartDate: &start, Deadline: &deadline}, "objective-date-admin")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := svc.Update(context.Background(), created.Objective.ID, model.UpdateObjectiveRequest{PlannedStartDateSet: true, DeadlineSet: true}, "objective-date-admin")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Objective.PlannedStartDate != nil || updated.Objective.Deadline != nil {
		t.Fatalf("dates were not cleared: %#v", updated.Objective)
	}
}

func TestPMObjectiveServiceCreateAndUpdateAssociationsAreAtomic(t *testing.T) {
	db := newTestDB(t)
	workspaceID := "ws-objective-atomic"
	seedUser(t, db, "objective-atomic-admin", "objective-atomic@example.com", "Objective Atomic Admin", "hash")
	seedWorkspace(t, db, workspaceID, "Objective Atomic", "objective-atomic", "objective-atomic-admin")
	seedWorkspaceMember(t, db, "objective-atomic-member", workspaceID, "objective-atomic-admin", "objective-atomic@example.com", "Objective Atomic Admin", model.RoleAdmin)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, "objective-atomic-team", workspaceID, "Atomic Team", now, now)
	activity := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := NewPMObjectiveService(repository.NewPMObjectiveRepository(db), repository.NewPMKeyResultRepository(db), repository.NewPMLabelRepository(db), nil, repository.NewWorkspaceRepository(db), activity, nil, nil)

	if _, err := svc.Create(context.Background(), model.CreateObjectiveRequest{
		WorkspaceID: workspaceID, Name: "Must Roll Back", ObjectiveType: model.PMObjectiveTypeStrategic,
		TeamIDs: []string{"objective-atomic-team", "objective-atomic-team"},
	}, "objective-atomic-admin"); err == nil {
		t.Fatal("expected duplicate association create to fail")
	}
	var createCount int64
	if err := db.Model(&model.PMObjective{}).Where("workspace_id = ? AND name = ?", workspaceID, "Must Roll Back").Count(&createCount).Error; err != nil || createCount != 0 {
		t.Fatalf("failed create left objective row: count=%d err=%v", createCount, err)
	}

	created, err := svc.Create(context.Background(), model.CreateObjectiveRequest{
		WorkspaceID: workspaceID, Name: "Original", ObjectiveType: model.PMObjectiveTypeStrategic,
		TeamIDs: []string{"objective-atomic-team"},
	}, "objective-atomic-admin")
	if err != nil {
		t.Fatalf("seed objective: %v", err)
	}
	changed := "Changed"
	if _, err := svc.Update(context.Background(), created.Objective.ID, model.UpdateObjectiveRequest{
		Name: &changed, TeamIDs: []string{"objective-atomic-team", "objective-atomic-team"},
	}, "objective-atomic-admin"); err == nil {
		t.Fatal("expected duplicate association update to fail")
	}
	persisted, err := svc.GetByID(context.Background(), created.Objective.ID, workspaceID)
	if err != nil {
		t.Fatalf("reload objective: %v", err)
	}
	if persisted.Objective.Name != "Original" || len(persisted.Teams) != 1 || persisted.Teams[0] != "objective-atomic-team" {
		t.Fatalf("failed update was partially committed: %#v", persisted)
	}
}

func TestPMObjectiveServiceUpdateKeyResultRejectsOrphanParent(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewPMKeyResultRepository(db)
	svc := NewPMObjectiveService(repository.NewPMObjectiveRepository(db), repo, repository.NewPMLabelRepository(db), nil, repository.NewWorkspaceRepository(db), nil, nil, nil)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO pm_key_results (id, objective_id, name, result_type, initial_value, current_value, target_value, progress, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "orphan-key-result", "missing-objective", "Orphan", model.PMKeyResultTypeNumeric, 0, 0, 100, 0, now, now)
	current := 50.0
	if _, err := svc.UpdateKeyResult(context.Background(), "orphan-key-result", model.UpdateKeyResultRequest{CurrentValue: &current}, "actor"); err == nil || !strings.Contains(err.Error(), "objective not found") {
		t.Fatalf("orphan update error = %v", err)
	}
	persisted, err := repo.GetByID(context.Background(), "orphan-key-result")
	if err != nil || persisted.CurrentValue != 0 {
		t.Fatalf("orphan key result was changed: kr=%#v err=%v", persisted, err)
	}
}

func TestComputeSuggestedHealth(t *testing.T) {
	t.Parallel()

	makeObjective := func(start, end time.Time, keyResultCount int, avgProgress float64) *model.ObjectiveWithDetails {
		return &model.ObjectiveWithDetails{
			Objective: model.PMObjective{
				PlannedStartDate: &start,
				Deadline:         &end,
			},
			Stats: model.PMObjectiveStats{
				KeyResultCount:  keyResultCount,
				KeyResultAvgPct: avgProgress,
			},
		}
	}

	t.Run("no dates returns on_track", func(t *testing.T) {
		objective := &model.ObjectiveWithDetails{
			Stats: model.PMObjectiveStats{KeyResultCount: 3, KeyResultAvgPct: 0},
		}
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("no key results returns on_track", func(t *testing.T) {
		objective := &model.ObjectiveWithDetails{
			Stats: model.PMObjectiveStats{KeyResultCount: 0},
		}
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("before start returns on_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 27, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 4, 0)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("start day with no progress remains on_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 2, 0)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 18, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("deadline day is not automatically overdue", func(t *testing.T) {
		start := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 2, 50)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 14, 9, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}
	})

	t.Run("after deadline with incomplete work returns off_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 12, 0, 0, 0, 0, time.UTC)
		objective := makeObjective(start, end, 4, 75)
		result := computeSuggestedHealthAt(objective, time.Date(2026, time.March, 13, 8, 0, 0, 0, time.UTC))
		if result != model.PMObjectiveHealthOffTrack {
			t.Errorf("got %q, want %q", result, model.PMObjectiveHealthOffTrack)
		}
	})

	t.Run("gap thresholds map to on_track at_risk and off_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 19, 0, 0, 0, 0, time.UTC)
		now := time.Date(2026, time.March, 15, 12, 0, 0, 0, time.UTC)

		onTrack := makeObjective(start, end, 4, 40) // expected 50%, actual 40%, gap 10
		if result := computeSuggestedHealthAt(onTrack, now); result != model.PMObjectiveHealthOnTrack {
			t.Errorf("on_track got %q, want %q", result, model.PMObjectiveHealthOnTrack)
		}

		atRisk := makeObjective(start, end, 4, 30) // expected 50%, actual 30%, gap 20
		if result := computeSuggestedHealthAt(atRisk, now); result != model.PMObjectiveHealthAtRisk {
			t.Errorf("at_risk got %q, want %q", result, model.PMObjectiveHealthAtRisk)
		}

		offTrack := makeObjective(start, end, 4, 20) // expected 50%, actual 20%, gap 30
		if result := computeSuggestedHealthAt(offTrack, now); result != model.PMObjectiveHealthOffTrack {
			t.Errorf("off_track got %q, want %q", result, model.PMObjectiveHealthOffTrack)
		}
	})
}

func TestPMObjectiveService_Create_ReassignsTemporaryAttachmentIDs(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-objective-attachments"
	userID := "user-objective-attachments"

	seedUser(t, db, userID, "objective@test.com", "Objective User", "hash")
	seedWorkspace(t, db, workspaceID, "Objective Workspace", "objective-ws", userID)
	seedWorkspaceMember(t, db, "member-objective-attachments", workspaceID, userID, "objective@test.com", "Objective User", model.RoleAdmin)

	seedTemporaryAttachment(t, db, "attachment-objective-1", workspaceID, workspaceID, userID)

	svc := NewPMObjectiveService(
		repository.NewPMObjectiveRepository(db),
		repository.NewPMKeyResultRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewPMAttachmentRepository(db),
		repository.NewWorkspaceRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
	)

	objective, err := svc.Create(ctx, model.CreateObjectiveRequest{
		WorkspaceID:   workspaceID,
		Name:          "Objective With Image",
		ObjectiveType: model.PMObjectiveTypeTactical,
		AttachmentIDs: []string{"attachment-objective-1"},
	}, userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	attachmentRepo := repository.NewPMAttachmentRepository(db)
	attachment, err := attachmentRepo.GetByID(ctx, "attachment-objective-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if attachment == nil {
		t.Fatal("expected attachment")
	}
	if attachment.EntityType != "objective" {
		t.Fatalf("entity_type = %q, want %q", attachment.EntityType, "objective")
	}
	if attachment.EntityID != objective.Objective.ID {
		t.Fatalf("entity_id = %q, want %q", attachment.EntityID, objective.Objective.ID)
	}
}

func TestKeyResultValueHistory(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedUser(t, db, "kr-author", "kr@example.com", "Arooj", "hash")
	seedWorkspace(t, db, "kr-workspace", "Results", "results", "kr-author")
	seedWorkspaceMember(t, db, "kr-member", "kr-workspace", "kr-author", "kr@example.com", "Arooj", model.RoleAdmin)
	svc := NewPMObjectiveService(repository.NewPMObjectiveRepository(db), repository.NewPMKeyResultRepository(db), repository.NewPMLabelRepository(db), nil, repository.NewWorkspaceRepository(db), NewPMActivityService(repository.NewPMActivityRepository(db)), nil, nil)
	obj, err := svc.Create(ctx, model.CreateObjectiveRequest{WorkspaceID: "kr-workspace", Name: "Results", ObjectiveType: model.PMObjectiveTypeStrategic}, "kr-author")
	if err != nil {
		t.Fatal(err)
	}
	kr, err := svc.CreateKeyResult(ctx, obj.Objective.ID, model.CreateKeyResultRequest{Name: "Signups", ResultType: "numeric", InitialValue: 13, CurrentValue: 13, TargetValue: 100}, "kr-author")
	if err != nil {
		t.Fatal(err)
	}
	value := 18.0
	if _, err = svc.UpdateKeyResult(ctx, kr.ID, model.UpdateKeyResultRequest{CurrentValue: &value}, "kr-author"); err != nil {
		t.Fatal(err)
	}
	entries, total, err := svc.ListKeyResultActivity(ctx, "kr-workspace", kr.ID, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil || total != 1 {
		t.Fatalf("history: total=%d err=%v", total, err)
	}
	entry := entries[0]
	if entry.Actor == nil || entry.Actor.FullName != "Arooj" || *entry.Activity.OldValue != "13" || *entry.Activity.NewValue != "18" {
		t.Fatalf("bad history: %+v", entry)
	}
	name := "Renamed signups"
	if _, err = svc.UpdateKeyResult(ctx, kr.ID, model.UpdateKeyResultRequest{Name: &name}, "kr-author"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateKeyResult(ctx, kr.ID, model.UpdateKeyResultRequest{CurrentValue: &value}, "kr-author"); err != nil {
		t.Fatal(err)
	}
	_, total, err = svc.ListKeyResultActivity(ctx, "kr-workspace", kr.ID, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil || total != 1 {
		t.Fatalf("settings or unchanged value added false history: total=%d err=%v", total, err)
	}
	if _, _, err = svc.ListKeyResultActivity(ctx, "other-workspace", kr.ID, model.PMPagination{}); err == nil {
		t.Fatal("cross-workspace history allowed")
	}
	mustExec(t, db, "CREATE TRIGGER fail_result_history BEFORE INSERT ON pm_activity_log WHEN NEW.entity_type = 'key_result' BEGIN SELECT RAISE(FAIL, 'history unavailable'); END")
	value = 20
	if _, err = svc.UpdateKeyResult(ctx, kr.ID, model.UpdateKeyResultRequest{CurrentValue: &value}, "kr-author"); err == nil {
		t.Fatal("expected history failure")
	}
	persisted, err := repository.NewPMKeyResultRepository(db).GetByID(ctx, kr.ID)
	if err != nil || persisted.CurrentValue != 18 {
		t.Fatalf("value was not rolled back: %+v %v", persisted, err)
	}
}

func TestComputeKeyResultProgressUsesBaselineAndTarget(t *testing.T) {
	for _, resultType := range []string{"numeric", "percent"} {
		for _, tc := range []struct{ start, current, target, want float64 }{
			{20, 35, 50, 50}, {60, 30, 10, 60}, {0, 110, 100, 100}, {20, 10, 50, 0}, {10, 10, 10, 0},
		} {
			got := computeKeyResultProgress(&model.PMKeyResult{ResultType: resultType, InitialValue: tc.start, CurrentValue: tc.current, TargetValue: tc.target})
			if got != tc.want {
				t.Errorf("%s %+v: got %v", resultType, tc, got)
			}
		}
	}
}

func TestPercentageKeyResultProgressBackfill(t *testing.T) {
	db := newTestDB(t)
	sql, err := os.ReadFile("../dbmigrate/sql/202609270001_key_result_percentage_progress.sql")
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, tc := range []struct{ start, current, target, want float64 }{
		{20, 35, 50, 50}, {60, 30, 10, 60}, {0, 110, 100, 100}, {20, 10, 50, 0}, {10, 10, 10, 0},
	} {
		id := fmt.Sprintf("percent-%d", i)
		row := model.PMKeyResult{ID: id, ObjectiveID: "objective", Name: "Percentage", ResultType: "percent", InitialValue: tc.start, CurrentValue: tc.current, TargetValue: tc.target, Progress: tc.current, UpdatedAt: when}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		for run := 0; run < 2; run++ {
			if err := db.Exec(string(sql)).Error; err != nil {
				t.Fatal(err)
			}
			var got model.PMKeyResult
			if err := db.First(&got, "id = ?", id).Error; err != nil {
				t.Fatal(err)
			}
			if got.Progress != tc.want || !got.UpdatedAt.Equal(when) || got.CurrentValue != tc.current {
				t.Fatalf("bad backfill: %+v", got)
			}
		}
	}
}
