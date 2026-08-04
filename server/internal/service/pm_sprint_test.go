package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// newSprintTestService sets up a PMSprintService backed by an in-memory SQLite DB.
// It seeds a workspace and returns the service, the DB, and the workspace ID.
func newSprintTestService(t *testing.T) (*PMSprintService, string) {
	t.Helper()
	svc, _, wsID := newSprintTestEnvWithDB(t)
	return svc, wsID
}

func newSprintTestEnvWithDB(t *testing.T) (*PMSprintService, *gorm.DB, string) {
	t.Helper()

	db := newTestDB(t)
	seedWorkspace(t, db, "ws-sprint", "Sprint WS", "sprint-ws", "owner-1")

	sprintRepo := repository.NewPMSprintRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)

	svc := NewPMSprintService(
		sprintRepo,
		labelRepo,
		repository.NewPMAttachmentRepository(db),
		repository.NewWorkspaceRepository(db),
		repository.NewSettingsRepository(db),
		activityService,
		nil,
		nil,
		nil,
	)
	return svc, db, "ws-sprint"
}

// makeSprintDates returns a start and end date offset by the given number of days
// from the reference time. Useful for creating non-overlapping sprint ranges.
func makeSprintDates(ref time.Time, startOffsetDays, endOffsetDays int) (time.Time, time.Time) {
	start := ref.AddDate(0, 0, startOffsetDays)
	end := ref.AddDate(0, 0, endOffsetDays)
	return start, end
}

func TestCreateSprint(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint 1",
		StartDate:   start,
		EndDate:     end,
	}

	result, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result == nil {
		t.Fatal("Create returned nil")
	}
	if result.Sprint.Name != "Sprint 1" {
		t.Fatalf("name = %q, want %q", result.Sprint.Name, "Sprint 1")
	}
	if result.Sprint.WorkspaceID != wsID {
		t.Fatalf("workspace_id = %q, want %q", result.Sprint.WorkspaceID, wsID)
	}
	if result.Sprint.ID == "" {
		t.Fatal("sprint ID is empty")
	}
}

func TestCreateSprint_MissingWorkspaceID(t *testing.T) {
	t.Parallel()
	svc, _ := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: "",
		Name:        "Sprint X",
		StartDate:   start,
		EndDate:     end,
	}

	_, err := svc.Create(ctx, req, "actor-1")
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestCreateSprint_MissingName(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "   ",
		StartDate:   start,
		EndDate:     end,
	}

	_, err := svc.Create(ctx, req, "actor-1")
	if err == nil {
		t.Fatal("expected error for blank name")
	}
}

func TestCreateSprint_EndDateBeforeStartDate(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 14, 1) // end before start
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Bad Dates",
		StartDate:   start,
		EndDate:     end,
	}

	_, err := svc.Create(ctx, req, "actor-1")
	if err == nil {
		t.Fatal("expected error for end_date before start_date")
	}
}

func TestCreateSprint_OverlapRejected(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req1 := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint A",
		StartDate:   start,
		EndDate:     end,
	}
	if _, err := svc.Create(ctx, req1, "actor-1"); err != nil {
		t.Fatalf("Create first sprint: %v", err)
	}

	// Create overlapping sprint (same date range, no team)
	req2 := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint B Overlap",
		StartDate:   start.AddDate(0, 0, 2),
		EndDate:     end.AddDate(0, 0, -2),
	}
	_, err := svc.Create(ctx, req2, "actor-1")
	if err == nil {
		t.Fatal("expected error for overlapping sprint date range")
	}
}

func TestCreateSprint_WithDescription(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	desc := "A sprint with a description"
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Desc",
		Description: &desc,
		StartDate:   start,
		EndDate:     end,
	}

	result, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Sprint.Description == nil || *result.Sprint.Description != desc {
		t.Fatalf("description = %v, want %q", result.Sprint.Description, desc)
	}
}

func TestCreateSprint_InvalidLabelDoesNotPersistSprint(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	seedWorkspace(t, db, "ws-sprint-other", "Other Sprint WS", "other-sprint-ws", "owner-2")
	if err := db.Create(&model.PMLabel{ID: "label-other", WorkspaceID: "ws-sprint-other", Name: "Other"}).Error; err != nil {
		t.Fatalf("seed cross-workspace label: %v", err)
	}

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	_, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Must not persist",
		StartDate:   start,
		EndDate:     end,
		LabelIDs:    []string{"label-other"},
	}, "actor-1")
	if err == nil {
		t.Fatal("expected cross-workspace label error")
	}

	var count int64
	if err := db.Model(&model.PMSprint{}).Where("workspace_id = ? AND name = ?", wsID, "Must not persist").Count(&count).Error; err != nil {
		t.Fatalf("count persisted sprints: %v", err)
	}
	if count != 0 {
		t.Fatalf("invalid create persisted %d sprint rows, want 0", count)
	}
}

func TestCreateSprint_DeduplicatesLabelIDs(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	if err := db.Create(&model.PMLabel{ID: "label-sprint-deduplicate", WorkspaceID: wsID, Name: "Deduplicate"}).Error; err != nil {
		t.Fatalf("seed label: %v", err)
	}

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Deduplicated sprint",
		StartDate:   start,
		EndDate:     end,
		LabelIDs:    []string{"label-sprint-deduplicate", "label-sprint-deduplicate"},
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(created.Labels) != 1 || created.Labels[0].ID != "label-sprint-deduplicate" {
		t.Fatalf("labels = %#v, want one deduplicated label", created.Labels)
	}
}

func TestCreateSprint_RollsBackSprintWhenLabelWriteFails(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	if err := db.Create(&model.PMLabel{ID: "label-sprint-create-failure", WorkspaceID: wsID, Name: "Failure"}).Error; err != nil {
		t.Fatalf("seed label: %v", err)
	}
	if err := db.Exec(`
		CREATE TRIGGER fail_sprint_create_label_insert
		BEFORE INSERT ON pm_sprint_labels
		WHEN NEW.label_id = 'label-sprint-create-failure'
		BEGIN
			SELECT RAISE(ABORT, 'forced sprint label insert failure');
		END
	`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	_, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Must roll back",
		StartDate:   start,
		EndDate:     end,
		LabelIDs:    []string{"label-sprint-create-failure"},
	}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "forced sprint label insert failure") {
		t.Fatalf("Create error = %v, want forced label failure", err)
	}

	var sprintCount int64
	if err := db.Model(&model.PMSprint{}).Where("workspace_id = ? AND name = ?", wsID, "Must roll back").Count(&sprintCount).Error; err != nil {
		t.Fatalf("count sprints: %v", err)
	}
	if sprintCount != 0 {
		t.Fatalf("failed label write left %d sprint rows, want 0", sprintCount)
	}
	var activityCount int64
	if err := db.Model(&model.PMActivityLog{}).Where("workspace_id = ?", wsID).Count(&activityCount).Error; err != nil {
		t.Fatalf("count activity: %v", err)
	}
	if activityCount != 0 {
		t.Fatalf("failed transaction emitted %d activity rows, want 0", activityCount)
	}
}

func TestCreateSprint_ReassignsTemporaryAttachmentIDs(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	seedTemporaryAttachment(t, db, "attachment-sprint-1", wsID, wsID, "actor-1")

	req := model.CreateSprintRequest{
		WorkspaceID:   wsID,
		Name:          "Sprint With Image",
		StartDate:     start,
		EndDate:       end,
		AttachmentIDs: []string{"attachment-sprint-1"},
	}

	result, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	attachmentRepo := repository.NewPMAttachmentRepository(db)
	attachment, err := attachmentRepo.GetByID(ctx, "attachment-sprint-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if attachment == nil {
		t.Fatal("expected attachment")
	}
	if attachment.EntityType != "sprint" {
		t.Fatalf("entity_type = %q, want %q", attachment.EntityType, "sprint")
	}
	if attachment.EntityID != result.Sprint.ID {
		t.Fatalf("entity_id = %q, want %q", attachment.EntityID, result.Sprint.ID)
	}
}

func TestListSprints(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	// Create two non-overlapping sprints
	start1, end1 := makeSprintDates(time.Now().UTC(), 1, 14)
	req1 := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint 1",
		StartDate:   start1,
		EndDate:     end1,
	}
	if _, err := svc.Create(ctx, req1, "actor-1"); err != nil {
		t.Fatalf("Create sprint 1: %v", err)
	}

	start2, end2 := makeSprintDates(time.Now().UTC(), 15, 28)
	req2 := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint 2",
		StartDate:   start2,
		EndDate:     end2,
	}
	if _, err := svc.Create(ctx, req2, "actor-1"); err != nil {
		t.Fatalf("Create sprint 2: %v", err)
	}

	// List all sprints
	filters := model.PMSprintListFilters{}
	sprints, err := svc.List(ctx, wsID, filters)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sprints) != 2 {
		t.Fatalf("List count = %d, want 2", len(sprints))
	}
}

func TestListPlanningWorkspace_GroupsSprintsAndBacklog(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	ctx := context.Background()

	const (
		teamIDValue  = "team-planning"
		workflowID   = "wf-planning"
		todoStateID  = "state-planning-todo"
		doingStateID = "state-planning-doing"
		doneStateID  = "state-planning-done"
	)
	teamID := teamIDValue

	seedPMSprintPlanningServiceState(t, db, todoStateID, workflowID, "Todo", model.PMStateTypeUnstarted, 0)
	seedPMSprintPlanningServiceState(t, db, doingStateID, workflowID, "Doing", model.PMStateTypeStarted, 1)
	seedPMSprintPlanningServiceState(t, db, doneStateID, workflowID, "Done", model.PMStateTypeDone, 2)

	now := time.Now().UTC()
	activeStart, activeEnd := makeSprintDates(now, -2, 5)
	upcomingStart, upcomingEnd := makeSprintDates(now, 8, 15)
	completedStart, completedEnd := makeSprintDates(now, -15, -8)

	active, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Active planning sprint",
		StartDate:   activeStart,
		EndDate:     activeEnd,
		TeamID:      &teamID,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create active sprint: %v", err)
	}
	if _, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Upcoming planning sprint",
		StartDate:   upcomingStart,
		EndDate:     upcomingEnd,
		TeamID:      &teamID,
	}, "actor-1"); err != nil {
		t.Fatalf("Create upcoming sprint: %v", err)
	}
	if _, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Completed planning sprint",
		StartDate:   completedStart,
		EndDate:     completedEnd,
		TeamID:      &teamID,
	}, "actor-1"); err != nil {
		t.Fatalf("Create completed sprint: %v", err)
	}

	seedPMSprintPlanningServiceStory(t, db, "story-active", wsID, workflowID, todoStateID, active.Sprint.ID, teamIDValue, "Active work", 2001, 1, 3)
	seedPMSprintPlanningServiceStory(t, db, "story-backlog", wsID, workflowID, doingStateID, "", teamIDValue, "Backlog work", 2002, 2, 5)
	seedPMSprintPlanningServiceStory(t, db, "story-backlog-done", wsID, workflowID, doneStateID, "", teamIDValue, "Backlog done", 2003, 3, 1)

	workspace, err := svc.ListPlanningWorkspace(ctx, wsID, model.PMSprintPlanningFilters{
		TeamID:           &teamID,
		IncludeCompleted: true,
	})
	if err != nil {
		t.Fatalf("ListPlanningWorkspace: %v", err)
	}

	if len(workspace.Buckets) != 3 {
		t.Fatalf("buckets = %d, want 3", len(workspace.Buckets))
	}
	if len(workspace.Buckets[0].Sprints) != 1 || workspace.Buckets[0].Sprints[0].Sprint.Name != "Active planning sprint" {
		t.Fatalf("active bucket = %+v", workspace.Buckets[0].Sprints)
	}
	if workspace.BacklogTotal != 1 {
		t.Fatalf("backlog_total = %d, want 1", workspace.BacklogTotal)
	}
}

func TestListSprints_EmptyWorkspaceID(t *testing.T) {
	t.Parallel()
	svc, _ := newSprintTestService(t)
	ctx := context.Background()

	_, err := svc.List(ctx, "", model.PMSprintListFilters{})
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
}

func seedPMSprintPlanningServiceState(t *testing.T, db *gorm.DB, id, workflowID, name, stateType string, position int) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workflowID, name, stateType, position, false, now, now,
	).Error; err != nil {
		t.Fatalf("seed workflow state: %v", err)
	}
}

func seedPMSprintPlanningServiceStory(t *testing.T, db *gorm.DB, id, workspaceID, workflowID, stateID, sprintID, teamID, name string, displayID, position, estimate int) {
	t.Helper()
	now := time.Now().UTC()
	var sprint any
	if sprintID != "" {
		sprint = sprintID
	}
	if err := db.Exec(
		`INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_id, workflow_state_id, sprint_id, team_id, estimate, position, priority, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		id, workspaceID, displayID, name, workflowID, stateID, sprint, teamID, estimate, position, model.PMTaskPriorityMedium, now, now,
	).Error; err != nil {
		t.Fatalf("seed planning story: %v", err)
	}
}

func TestListSprints_Empty(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	sprints, err := svc.List(ctx, wsID, model.PMSprintListFilters{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sprints) != 0 {
		t.Fatalf("List count = %d, want 0", len(sprints))
	}
}

func TestGetByID(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Get",
		StartDate:   start,
		EndDate:     end,
	}
	created, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.GetByID(ctx, created.Sprint.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.Sprint.ID != created.Sprint.ID {
		t.Fatalf("ID = %q, want %q", got.Sprint.ID, created.Sprint.ID)
	}
	if got.Sprint.Name != "Sprint Get" {
		t.Fatalf("Name = %q, want %q", got.Sprint.Name, "Sprint Get")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newSprintTestService(t)
	ctx := context.Background()

	_, err := svc.GetByID(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent sprint")
	}
}

func TestUpdateSprint(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Original",
		StartDate:   start,
		EndDate:     end,
	}
	created, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Sprint Updated"
	updateReq := model.UpdateSprintRequest{
		Name: &newName,
	}
	updated, err := svc.Update(ctx, created.Sprint.ID, updateReq, "actor-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Sprint.Name != "Sprint Updated" {
		t.Fatalf("Name = %q, want %q", updated.Sprint.Name, "Sprint Updated")
	}
}

func TestUpdateSprint_EmptyName(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint To Blank",
		StartDate:   start,
		EndDate:     end,
	}
	created, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	emptyName := "   "
	_, err = svc.Update(ctx, created.Sprint.ID, model.UpdateSprintRequest{Name: &emptyName}, "actor-1")
	if err == nil {
		t.Fatal("expected error for blank name update")
	}
}

func TestUpdateSprint_ChangeDescription(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Desc Update",
		StartDate:   start,
		EndDate:     end,
	}
	created, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newDesc := "Updated description"
	updated, err := svc.Update(ctx, created.Sprint.ID, model.UpdateSprintRequest{Description: &newDesc}, "actor-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Sprint.Description == nil || *updated.Sprint.Description != newDesc {
		t.Fatalf("description = %v, want %q", updated.Sprint.Description, newDesc)
	}
}

func TestUpdateSprint_InvalidLabelDoesNotPersistScalarChanges(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	seedWorkspace(t, db, "ws-sprint-other-update", "Other Sprint WS", "other-sprint-update-ws", "owner-2")
	if err := db.Create(&model.PMLabel{ID: "label-other-update", WorkspaceID: "ws-sprint-other-update", Name: "Other"}).Error; err != nil {
		t.Fatalf("seed cross-workspace label: %v", err)
	}

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Original name",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Must not persist"
	_, err = svc.Update(context.Background(), created.Sprint.ID, model.UpdateSprintRequest{
		Name:     &newName,
		LabelIDs: []string{"label-other-update"},
	}, "actor-1")
	if err == nil {
		t.Fatal("expected cross-workspace label error")
	}

	reloaded, err := repository.NewPMSprintRepository(db).GetWithStats(context.Background(), created.Sprint.ID)
	if err != nil {
		t.Fatalf("reload sprint: %v", err)
	}
	if reloaded == nil || reloaded.Sprint.Name != "Original name" {
		t.Fatalf("invalid update persisted scalar changes: %#v", reloaded)
	}
}

func TestUpdateSprint_RequiresManagementOfDestinationTeam(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"team-sprint-current", wsID, "Current Team", now, now)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"team-sprint-destination", wsID, "Destination Team", now, now)

	currentTeamID := "team-sprint-current"
	start, end := makeSprintDates(now, 1, 14)
	created, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Team move",
		StartDate:   start,
		EndDate:     end,
		TeamID:      &currentTeamID,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	actor := &authorization.Actor{
		UserID:      "manager-1",
		WorkspaceID: wsID,
		Role:        "manager",
		TeamMemberships: []authorization.TeamRole{
			{TeamID: currentTeamID, Role: "owner"},
		},
	}
	ctx := authorization.WithActor(context.Background(), actor)
	destinationTeamID := "team-sprint-destination"
	_, err = svc.Update(ctx, created.Sprint.ID, model.UpdateSprintRequest{TeamID: &destinationTeamID}, actor.UserID)
	if err == nil {
		t.Fatal("expected destination-team management error")
	}

	reloaded, err := repository.NewPMSprintRepository(db).GetWithStats(context.Background(), created.Sprint.ID)
	if err != nil {
		t.Fatalf("reload sprint: %v", err)
	}
	if reloaded == nil || reloaded.Sprint.TeamID == nil || *reloaded.Sprint.TeamID != currentTeamID {
		t.Fatalf("unauthorized team move persisted: %#v", reloaded)
	}
}

func TestUpdateSprint_TeamChangeValidatesPreservedLabels(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"team-sprint-label-source", wsID, "Source Team", now, now)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"team-sprint-label-destination", wsID, "Destination Team", now, now)
	sourceTeamID := "team-sprint-label-source"
	destinationTeamID := "team-sprint-label-destination"
	if err := db.Create(&model.PMLabel{
		ID: "label-sprint-source-team", WorkspaceID: wsID, TeamID: &sourceTeamID, Name: "Source only",
	}).Error; err != nil {
		t.Fatalf("seed team label: %v", err)
	}
	start, end := makeSprintDates(now, 1, 14)
	created, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Preserve labels",
		StartDate:   start,
		EndDate:     end,
		TeamID:      &sourceTeamID,
		LabelIDs:    []string{"label-sprint-source-team"},
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.Update(context.Background(), created.Sprint.ID, model.UpdateSprintRequest{TeamID: &destinationTeamID}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "does not belong to the selected team scope") {
		t.Fatalf("Update error = %v, want preserved-label scope failure", err)
	}
	reloaded, err := repository.NewPMSprintRepository(db).GetWithStats(context.Background(), created.Sprint.ID)
	if err != nil {
		t.Fatalf("reload sprint: %v", err)
	}
	if reloaded == nil || reloaded.Sprint.TeamID == nil || *reloaded.Sprint.TeamID != sourceTeamID {
		t.Fatalf("invalid team move persisted: %#v", reloaded)
	}
}

func TestUpdateSprint_RollsBackScalarAndLabelsWhenLabelWriteFails(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	for _, label := range []model.PMLabel{
		{ID: "label-sprint-existing", WorkspaceID: wsID, Name: "Existing"},
		{ID: "label-sprint-update-failure", WorkspaceID: wsID, Name: "Failure"},
	} {
		if err := db.Create(&label).Error; err != nil {
			t.Fatalf("seed label %s: %v", label.ID, err)
		}
	}
	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Original atomic name",
		StartDate:   start,
		EndDate:     end,
		LabelIDs:    []string{"label-sprint-existing"},
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	var activityBefore int64
	if err := db.Model(&model.PMActivityLog{}).Where("workspace_id = ?", wsID).Count(&activityBefore).Error; err != nil {
		t.Fatalf("count activity before update: %v", err)
	}
	if err := db.Exec(`
		CREATE TRIGGER fail_sprint_update_label_insert
		BEFORE INSERT ON pm_sprint_labels
		WHEN NEW.label_id = 'label-sprint-update-failure'
		BEGIN
			SELECT RAISE(ABORT, 'forced sprint label update failure');
		END
	`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	updatedName := "Must roll back"
	_, err = svc.Update(context.Background(), created.Sprint.ID, model.UpdateSprintRequest{
		Name:     &updatedName,
		LabelIDs: []string{"label-sprint-update-failure"},
	}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "forced sprint label update failure") {
		t.Fatalf("Update error = %v, want forced label failure", err)
	}

	reloaded, err := repository.NewPMSprintRepository(db).GetWithStats(context.Background(), created.Sprint.ID)
	if err != nil {
		t.Fatalf("reload sprint: %v", err)
	}
	if reloaded == nil || reloaded.Sprint.Name != "Original atomic name" {
		t.Fatalf("failed label write persisted scalar update: %#v", reloaded)
	}
	if len(reloaded.Labels) != 1 || reloaded.Labels[0].ID != "label-sprint-existing" {
		t.Fatalf("failed label write changed labels: %#v", reloaded.Labels)
	}
	var activityAfter int64
	if err := db.Model(&model.PMActivityLog{}).Where("workspace_id = ?", wsID).Count(&activityAfter).Error; err != nil {
		t.Fatalf("count activity after update: %v", err)
	}
	if activityAfter != activityBefore {
		t.Fatalf("failed transaction changed activity count from %d to %d", activityBefore, activityAfter)
	}
}

func TestUpdateSprint_DeduplicatesLabelIDs(t *testing.T) {
	t.Parallel()
	svc, db, wsID := newSprintTestEnvWithDB(t)
	if err := db.Create(&model.PMLabel{ID: "label-sprint-update-deduplicate", WorkspaceID: wsID, Name: "Deduplicate"}).Error; err != nil {
		t.Fatalf("seed label: %v", err)
	}
	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Update deduplication",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(context.Background(), created.Sprint.ID, model.UpdateSprintRequest{
		LabelIDs: []string{"label-sprint-update-deduplicate", "label-sprint-update-deduplicate"},
	}, "actor-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.Labels) != 1 || updated.Labels[0].ID != "label-sprint-update-deduplicate" {
		t.Fatalf("labels = %#v, want one deduplicated label", updated.Labels)
	}
}

func TestUpdateSprint_ChangeDates(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Date Update",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newStart := start.AddDate(0, 0, 2)
	newEnd := end.AddDate(0, 0, 5)
	updated, err := svc.Update(ctx, created.Sprint.ID, model.UpdateSprintRequest{
		StartDate: &newStart,
		EndDate:   &newEnd,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Sprint.StartDate.Day() != newStart.Day() {
		t.Fatalf("start_date day = %d, want %d", updated.Sprint.StartDate.Day(), newStart.Day())
	}
	if updated.Sprint.EndDate.Day() != newEnd.Day() {
		t.Fatalf("end_date day = %d, want %d", updated.Sprint.EndDate.Day(), newEnd.Day())
	}
}

func TestUpdateSprint_ArchiveFlag(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Archive",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	archived := true
	updated, err := svc.Update(ctx, created.Sprint.ID, model.UpdateSprintRequest{Archived: &archived}, "actor-1")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.Sprint.Archived {
		t.Fatal("expected sprint to be archived")
	}
}

func TestUpdateSprint_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newSprintTestService(t)
	ctx := context.Background()

	newName := "Ghost"
	_, err := svc.Update(ctx, "nonexistent-id", model.UpdateSprintRequest{Name: &newName}, "actor-1")
	if err == nil {
		t.Fatal("expected error for nonexistent sprint")
	}
}

func TestDeleteSprint(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Delete",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(ctx, created.Sprint.ID, "actor-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone
	_, err = svc.GetByID(ctx, created.Sprint.ID)
	if err == nil {
		t.Fatal("expected error after deleting sprint")
	}
}

func TestDeleteSprint_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newSprintTestService(t)
	ctx := context.Background()

	err := svc.Delete(ctx, "nonexistent-id", "actor-1")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent sprint")
	}
}

func TestGetCurrentSprint(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	// Create a sprint whose date range encompasses today
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -3)
	end := now.AddDate(0, 0, 10)
	_, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Current Sprint",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	current, err := svc.GetCurrentSprint(ctx, wsID, nil)
	if err != nil {
		t.Fatalf("GetCurrentSprint: %v", err)
	}
	if current == nil {
		t.Fatal("GetCurrentSprint returned nil")
	}
	if current.Name != "Current Sprint" {
		t.Fatalf("Name = %q, want %q", current.Name, "Current Sprint")
	}
}

func TestGetCurrentSprint_NoActiveSprint(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	// Create a sprint entirely in the future
	start, end := makeSprintDates(time.Now().UTC(), 30, 44)
	_, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Future Sprint",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	current, err := svc.GetCurrentSprint(ctx, wsID, nil)
	if err != nil {
		t.Fatalf("GetCurrentSprint: %v", err)
	}
	if current != nil {
		t.Fatalf("expected nil for no current sprint, got %q", current.Name)
	}
}

func TestGetCurrentSprint_EmptyWorkspaceID(t *testing.T) {
	t.Parallel()
	svc, _ := newSprintTestService(t)
	ctx := context.Background()

	_, err := svc.GetCurrentSprint(ctx, "", nil)
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
}

func TestGetCurrentSprint_WithTeamID(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	seedWorkspace(t, db, "ws-team", "Team WS", "team-ws", "owner-1")
	now := time.Now()
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"team-1", "ws-team", "Alpha Team", now, now)

	sprintRepo := repository.NewPMSprintRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)
	svc := NewPMSprintService(
		sprintRepo,
		labelRepo,
		repository.NewPMAttachmentRepository(db),
		repository.NewWorkspaceRepository(db),
		repository.NewSettingsRepository(db),
		activityService,
		nil,
		nil,
		nil,
	)

	ctx := context.Background()
	teamID := "team-1"
	start := time.Now().UTC().AddDate(0, 0, -2)
	end := time.Now().UTC().AddDate(0, 0, 12)

	_, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: "ws-team",
		Name:        "Team Sprint",
		StartDate:   start,
		EndDate:     end,
		TeamID:      &teamID,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Should find sprint when filtering by the correct team
	current, err := svc.GetCurrentSprint(ctx, "ws-team", &teamID)
	if err != nil {
		t.Fatalf("GetCurrentSprint with team: %v", err)
	}
	if current == nil {
		t.Fatal("expected current sprint for team, got nil")
	}
	if current.Name != "Team Sprint" {
		t.Fatalf("Name = %q, want %q", current.Name, "Team Sprint")
	}

	// Should NOT find sprint for a different team
	otherTeam := "team-other"
	current, err = svc.GetCurrentSprint(ctx, "ws-team", &otherTeam)
	if err != nil {
		t.Fatalf("GetCurrentSprint with other team: %v", err)
	}
	if current != nil {
		t.Fatal("expected nil for different team, got a sprint")
	}
}

func TestListTasks_Empty(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint No Tasks",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tasks, err := svc.ListTasks(ctx, created.Sprint.ID)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("ListTasks count = %d, want 0", len(tasks))
	}
}

func TestComputeStats_EmptySprint(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Stats",
		StartDate:   start,
		EndDate:     end,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	stats, err := svc.ComputeStats(ctx, created.Sprint.ID)
	if err != nil {
		t.Fatalf("ComputeStats: %v", err)
	}
	if stats.TaskCount != 0 {
		t.Fatalf("TaskCount = %d, want 0", stats.TaskCount)
	}
	if stats.TotalPoints != 0 {
		t.Fatalf("TotalPoints = %d, want 0", stats.TotalPoints)
	}
	if stats.DoneTaskCount != 0 {
		t.Fatalf("DoneTaskCount = %d, want 0", stats.DoneTaskCount)
	}
	if stats.DonePoints != 0 {
		t.Fatalf("DonePoints = %d, want 0", stats.DonePoints)
	}
}

func TestCreateSprint_NameTrimmed(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	req := model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "  Sprint Trimmed  ",
		StartDate:   start,
		EndDate:     end,
	}

	result, err := svc.Create(ctx, req, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Sprint.Name != "Sprint Trimmed" {
		t.Fatalf("Name = %q, want %q", result.Sprint.Name, "Sprint Trimmed")
	}
}

func TestCreateSprint_CreatedBySet(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	result, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Actor",
		StartDate:   start,
		EndDate:     end,
	}, "actor-42")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Sprint.CreatedBy == nil || *result.Sprint.CreatedBy != "actor-42" {
		t.Fatalf("CreatedBy = %v, want %q", result.Sprint.CreatedBy, "actor-42")
	}
}

func TestCreateSprint_EmptyActorID(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	start, end := makeSprintDates(time.Now().UTC(), 1, 14)
	result, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint No Actor",
		StartDate:   start,
		EndDate:     end,
	}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Sprint.CreatedBy != nil {
		t.Fatalf("CreatedBy = %v, want nil", result.Sprint.CreatedBy)
	}
}

func TestListSprints_FilterByArchived(t *testing.T) {
	t.Parallel()
	svc, wsID := newSprintTestService(t)
	ctx := context.Background()

	// Create two sprints, archive one
	start1, end1 := makeSprintDates(time.Now().UTC(), 1, 14)
	created, err := svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint Active",
		StartDate:   start1,
		EndDate:     end1,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create sprint 1: %v", err)
	}

	start2, end2 := makeSprintDates(time.Now().UTC(), 15, 28)
	_, err = svc.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: wsID,
		Name:        "Sprint To Archive",
		StartDate:   start2,
		EndDate:     end2,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create sprint 2: %v", err)
	}

	// Archive the first one
	archived := true
	_, err = svc.Update(ctx, created.Sprint.ID, model.UpdateSprintRequest{Archived: &archived}, "actor-1")
	if err != nil {
		t.Fatalf("Update (archive): %v", err)
	}

	// List only non-archived
	notArchived := false
	sprints, err := svc.List(ctx, wsID, model.PMSprintListFilters{Archived: &notArchived})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sprints) != 1 {
		t.Fatalf("List count = %d, want 1 (only non-archived)", len(sprints))
	}
	if sprints[0].Sprint.Name != "Sprint To Archive" {
		t.Fatalf("Name = %q, want %q", sprints[0].Sprint.Name, "Sprint To Archive")
	}
}
