package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildEpicActivityChangesReportsMeaningfulFields(t *testing.T) {
	oldDeadline := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	newDeadline := time.Date(2026, time.August, 28, 0, 0, 0, 0, time.UTC)
	oldState := "state-todo"
	newState := "state-progress"
	oldDescription := "Old description"
	newDescription := "New description"

	before := model.PMEpic{
		Name: "Old title", Description: &oldDescription, EpicStateID: &oldState,
		Deadline: &oldDeadline, Health: model.PMEpicHealthOnTrack,
	}
	after := model.PMEpic{
		Name: "New title", Description: &newDescription, EpicStateID: &newState,
		Deadline: &newDeadline, Health: model.PMEpicHealthAtRisk,
	}

	changes := buildEpicActivityChanges(before, after)
	byField := make(map[string]epicActivityChange, len(changes))
	for _, change := range changes {
		byField[change.fieldName] = change
	}

	if got := *byField["name"].oldValue; got != "Old title" {
		t.Fatalf("old title = %q", got)
	}
	if got := *byField["name"].newValue; got != "New title" {
		t.Fatalf("new title = %q", got)
	}
	if byField["description"].oldValue != nil || byField["description"].newValue != nil {
		t.Fatal("description activity must not retain description content")
	}
	if got := *byField["deadline"].oldValue; got != "2026-08-20" {
		t.Fatalf("old deadline = %q", got)
	}
	if got := *byField["deadline"].newValue; got != "2026-08-28" {
		t.Fatalf("new deadline = %q", got)
	}
	if got := *byField["health"].newValue; got != model.PMEpicHealthAtRisk {
		t.Fatalf("new health = %q", got)
	}
}

func TestBuildEpicActivityChangesIgnoresNoOpsAndPosition(t *testing.T) {
	before := model.PMEpic{Name: "Same", Position: 1, Health: model.PMEpicHealthNone}
	after := before
	after.Position = 99

	if changes := buildEpicActivityChanges(before, after); len(changes) != 0 {
		t.Fatalf("position-only update produced activity: %#v", changes)
	}
}

func TestPMEpicServiceUpdateLogsStructuredActivityWithoutGenericRow(t *testing.T) {
	svc, db, workspaceID, userID := newEpicTestEnvWithDB(t)
	ctx := context.Background()
	created := createTestEpic(t, svc, workspaceID, userID, "Original title")

	stateID := "state-epic-activity"
	mustExec(t, db, `INSERT INTO pm_epic_workflow_states (id, workspace_id, name, state_type, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		stateID, workspaceID, "In Progress", model.PMStateTypeStarted, 1, time.Now(), time.Now())
	name := "Updated title"
	if _, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{Name: &name, EpicStateID: &stateID}, userID); err != nil {
		t.Fatalf("Update: %v", err)
	}

	entries, _, err := svc.ListActivity(ctx, created.Epic.ID, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	fields := make(map[string]model.PMActivityLog)
	for _, entry := range entries {
		if entry.Activity.Action == "updated" && entry.Activity.FieldName == nil {
			t.Fatal("found legacy fieldless updated activity")
		}
		if entry.Activity.FieldName != nil {
			fields[*entry.Activity.FieldName] = entry.Activity
		}
	}
	if fields["name"].OldValue == nil || *fields["name"].OldValue != "Original title" || fields["name"].NewValue == nil || *fields["name"].NewValue != name {
		t.Fatalf("name activity = %#v", fields["name"])
	}
	stateMetadata := make(map[string]interface{})
	if err := json.Unmarshal(fields["epic_state_id"].Metadata, &stateMetadata); err != nil {
		t.Fatalf("unmarshal state metadata: %v", err)
	}
	if got, _ := stateMetadata["new_label"].(string); got != "In Progress" {
		t.Fatalf("state new_label = %q, metadata = %#v", got, stateMetadata)
	}

	if _, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{Name: &name}, userID); err != nil {
		t.Fatalf("no-op Update: %v", err)
	}
	_, totalAfterNoOp, err := svc.ListActivity(ctx, created.Epic.ID, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListActivity after no-op: %v", err)
	}
	if totalAfterNoOp != int64(len(entries)) {
		t.Fatalf("no-op activity total = %d, want %d", totalAfterNoOp, len(entries))
	}
}
