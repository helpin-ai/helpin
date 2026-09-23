package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPMImportServicePreviewShortcut(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	existingExternalID := "85463"
	if err := db.Create(&model.PMTask{
		ID:              uuid.NewString(),
		WorkspaceID:     workspaceID,
		DisplayID:       1,
		Name:            "Existing Story",
		TaskType:        model.PMTaskTypeFeature,
		WorkflowID:      "wf-existing",
		WorkflowStateID: "state-existing",
		ExternalID:      &existingExternalID,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed existing story: %v", err)
	}

	resp, err := svc.PreviewShortcut(context.Background(), workspaceID, adminID, []byte(shortcutImportTestCSV()), "")
	if err != nil {
		t.Fatalf("preview shortcut: %v", err)
	}

	if resp.Summary.TotalTasks != 4 {
		t.Fatalf("expected 4 tasks, got %d", resp.Summary.TotalTasks)
	}
	if resp.Summary.DuplicateTasks != 1 {
		t.Fatalf("expected 1 duplicate task, got %d", resp.Summary.DuplicateTasks)
	}
	if resp.Summary.EpicsCount != 1 {
		t.Fatalf("expected 1 epic, got %d", resp.Summary.EpicsCount)
	}
	if resp.Summary.ObjectivesCount != 1 {
		t.Fatalf("expected 1 objective, got %d", resp.Summary.ObjectivesCount)
	}
	if resp.Summary.SprintsCount != 3 {
		t.Fatalf("expected 3 sprints, got %d", resp.Summary.SprintsCount)
	}
	if resp.Summary.ChecklistItemsCount != 4 {
		t.Fatalf("expected 4 checklist items, got %d", resp.Summary.ChecklistItemsCount)
	}
	if len(resp.Users) != 6 {
		t.Fatalf("expected 6 unique user emails, got %d", len(resp.Users))
	}

	var requesterMatched bool
	for _, user := range resp.Users {
		if user.Email == "requester@example.com" {
			requesterMatched = user.MatchedUserID != nil && *user.MatchedUserID != ""
		}
	}
	if !requesterMatched {
		t.Fatal("expected requester to auto-match")
	}

	if len(resp.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(resp.Workflows))
	}
	if resp.Workflows[0].TaskCount != 4 {
		t.Fatalf("expected workflow story count 4, got %d", resp.Workflows[0].TaskCount)
	}
}

func TestPMImportServiceExecuteShortcutAndIdempotency(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, _ := newImportTestService(t, db)

	req := model.ShortcutImportExecuteRequest{
		UserMappings: map[string]string{
			"owner.one@example.com": "user-owner-one",
			"owner.two@example.com": "user-owner-two",
		},
		WorkflowStateMappings: []model.ShortcutWorkflowStateMappingPayload{
			{
				ShortcutWorkflowName: "Product Development",
				Mode:                 "create_new",
				NewWorkflowName:      "Imported Product Development",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
					{ShortcutState: "Completed", NewStateName: "Completed", StateType: model.PMStateTypeDone, Position: 1},
				},
			},
		},
		Options: model.ShortcutImportOptions{
			ImportArchived:  true,
			ImportCompleted: true,
		},
	}

	result, totalRows, err := svc.executeShortcutImport(context.Background(), workspaceID, "user-admin", []byte(shortcutImportTestCSV()), req, "", "")
	if err != nil {
		t.Fatalf("execute shortcut import: %v", err)
	}

	if totalRows != 4 {
		t.Fatalf("expected 4 imported rows, got %d", totalRows)
	}
	if result.TeamsCreated != 2 {
		t.Fatalf("expected 2 teams created, got %d", result.TeamsCreated)
	}
	if result.WorkflowsCreated != 3 {
		t.Fatalf("expected 3 team-scoped workflows created, got %d", result.WorkflowsCreated)
	}
	if result.WorkflowStatesCreated != 6 {
		t.Fatalf("expected 6 team-scoped workflow states created, got %d", result.WorkflowStatesCreated)
	}
	if result.ObjectivesCreated != 1 || result.EpicsCreated != 1 || result.SprintsCreated != 3 {
		t.Fatalf("unexpected entity counts: objectives=%d epics=%d sprints=%d", result.ObjectivesCreated, result.EpicsCreated, result.SprintsCreated)
	}
	if result.TasksCreated != 4 || result.TasksSkipped != 0 {
		t.Fatalf("unexpected task counts: created=%d skipped=%d", result.TasksCreated, result.TasksSkipped)
	}
	if result.OwnerLinksCreated != 4 {
		t.Fatalf("expected 4 owner links created, got %d", result.OwnerLinksCreated)
	}
	if result.LabelLinksCreated != 5 {
		t.Fatalf("expected 5 task label links created, got %d", result.LabelLinksCreated)
	}
	if result.ChecklistItemsCreated != 4 {
		t.Fatalf("expected 4 checklist items created, got %d", result.ChecklistItemsCreated)
	}
	assertWarningContains(t, result.Warnings, "1 imported sprints had null dates because iteration names could not be parsed")
	assertWarningContains(t, result.Warnings, "1 tasks had unmapped requester emails")
	assertWarningContains(t, result.Warnings, "Owner email 'missing.owner@example.com' not mapped")

	assertImportState(t, db, workspaceID)

	secondResult, _, err := svc.executeShortcutImport(context.Background(), workspaceID, "user-admin", []byte(shortcutImportTestCSV()), req, "", "")
	if err != nil {
		t.Fatalf("execute shortcut import second run: %v", err)
	}
	if secondResult.TasksCreated != 0 {
		t.Fatalf("expected no new tasks on second import, got %d", secondResult.TasksCreated)
	}
	if secondResult.TasksSkipped != 4 {
		t.Fatalf("expected 4 skipped tasks on second import, got %d", secondResult.TasksSkipped)
	}
	if secondResult.EpicsCreated != 0 || secondResult.ObjectivesCreated != 0 || secondResult.SprintsCreated != 0 {
		t.Fatalf("expected no new deduped entities on second import, got epics=%d objectives=%d sprints=%d", secondResult.EpicsCreated, secondResult.ObjectivesCreated, secondResult.SprintsCreated)
	}
}

func TestPMImportServiceShortcutAPIEndToEndAndIdempotency(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)
	docsSpaceID := "docs-space-shortcut"
	if err := db.Create(&model.DocsSpace{
		ID:          docsSpaceID,
		WorkspaceID: workspaceID,
		Name:        "Imported Shortcut Docs",
		Slug:        "imported-shortcut-docs",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeInternal,
		CreatedBy:   adminID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed docs space: %v", err)
	}
	svc.SetDocsImportDependencies(
		NewDocsDocumentService(repository.NewDocsDocumentRepository(db), repository.NewDocsSpaceRepository(db), nil, false),
		NewDocsContentService(repository.NewDocsContentRepository(db), repository.NewDocsDocumentRepository(db), nil),
	)
	handler := newShortcutAPITestHandler(t)

	prevBaseURL := shortcutAPIBaseURL
	prevHTTPClientFactory := shortcutHTTPClientFactory
	shortcutAPIBaseURL = "https://shortcut.test"
	shortcutHTTPClientFactory = func() *http.Client {
		return &http.Client{Transport: handlerRoundTripper{handler: handler}}
	}
	t.Cleanup(func() {
		shortcutAPIBaseURL = prevBaseURL
		shortcutHTTPClientFactory = prevHTTPClientFactory
	})

	preview, err := svc.PreviewShortcutAPI(context.Background(), workspaceID, adminID, model.ShortcutAPIImportPreviewRequest{
		APIToken: "test-token",
		Options:  model.ShortcutImportOptions{ImportArchived: true, ImportCompleted: true, ImportDocs: true},
	})
	if err != nil {
		t.Fatalf("preview Shortcut API import: %v", err)
	}
	if preview.Summary.TotalTasks != 2 {
		t.Fatalf("expected 2 API stories in preview, got %d", preview.Summary.TotalTasks)
	}
	if preview.Summary.EpicsCount != 1 || preview.Summary.ObjectivesCount != 1 || preview.Summary.SprintsCount != 1 {
		t.Fatalf("unexpected preview hierarchy counts: epics=%d objectives=%d sprints=%d", preview.Summary.EpicsCount, preview.Summary.ObjectivesCount, preview.Summary.SprintsCount)
	}
	if len(preview.Workflows) != 1 || preview.Workflows[0].ID != "500" {
		t.Fatalf("expected workflow 500 in preview, got %+v", preview.Workflows)
	}
	if preview.Summary.DocsCount != 1 {
		t.Fatalf("expected 1 Shortcut doc in preview, got %d", preview.Summary.DocsCount)
	}

	req := model.ShortcutAPIImportExecuteRequest{
		APIToken: "test-token",
		UserMappings: map[string]string{
			"owner.one@example.com": "user-owner-one",
			"owner.two@example.com": "user-owner-two",
		},
		WorkflowStateMappings: []model.ShortcutWorkflowStateMappingPayload{
			{
				ShortcutWorkflowID:   "500",
				ShortcutWorkflowName: "Product Development",
				Mode:                 "create_new",
				NewWorkflowName:      "Product Development",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
					{ShortcutState: "Done", NewStateName: "Done", StateType: model.PMStateTypeDone, Position: 1},
				},
			},
		},
		Options: model.ShortcutImportOptions{ImportArchived: true, ImportCompleted: true, ImportDocs: true, DocsSpaceID: docsSpaceID},
	}

	result, totalRows, err := svc.executeShortcutAPIImport(context.Background(), workspaceID, adminID, req, "")
	if err != nil {
		t.Fatalf("execute Shortcut API import: %v", err)
	}
	if totalRows != 2 || result.TasksCreated != 2 {
		t.Fatalf("expected 2 API tasks created, got rows=%d created=%d", totalRows, result.TasksCreated)
	}
	if result.CommentsCreated != 1 {
		t.Fatalf("expected 1 comment created, got %d warnings=%v", result.CommentsCreated, result.Warnings)
	}
	if result.ChecklistItemsCreated != 1 {
		t.Fatalf("expected 1 checklist item created, got %d", result.ChecklistItemsCreated)
	}
	if result.TaskLinksCreated != 1 {
		t.Fatalf("expected 1 task link created, got %d", result.TaskLinksCreated)
	}
	if result.ExternalLinksCreated != 4 {
		t.Fatalf("expected 4 external links created, got %d", result.ExternalLinksCreated)
	}
	if result.SprintsCreated != 1 {
		t.Fatalf("expected 1 sprint created from Shortcut iteration, got %d", result.SprintsCreated)
	}
	if result.DocsCreated != 1 || result.DocsSkipped != 0 {
		t.Fatalf("expected 1 Shortcut doc created, got created=%d skipped=%d warnings=%v", result.DocsCreated, result.DocsSkipped, result.Warnings)
	}
	var docs []model.DocsDocument
	if err := db.Where("workspace_id = ?", workspaceID).Find(&docs).Error; err != nil {
		t.Fatalf("load imported docs: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "Shortcut Launch Plan" {
		t.Fatalf("expected imported Shortcut doc, got %+v", docs)
	}
	var docContent model.DocsContent
	if err := db.Where("document_id = ?", docs[0].ID).First(&docContent).Error; err != nil {
		t.Fatalf("load imported doc content: %v", err)
	}
	if docContent.ImportSourceSystem == nil || *docContent.ImportSourceSystem != shortcutDocsSourceSystem || docContent.ImportSourceObjectID == nil || *docContent.ImportSourceObjectID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected Shortcut provenance, got system=%v object=%v", docContent.ImportSourceSystem, docContent.ImportSourceObjectID)
	}

	var tasks []model.PMTask
	if err := db.Order("external_id ASC").Find(&tasks).Error; err != nil {
		t.Fatalf("load imported tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 imported tasks, got %d", len(tasks))
	}
	if tasks[0].ExternalID == nil || *tasks[0].ExternalID != "1001" {
		t.Fatalf("expected first task external id 1001, got %+v", tasks[0].ExternalID)
	}
	if tasks[0].TeamID == nil {
		t.Fatal("expected API group to map to a Helpin team")
	}
	for _, task := range tasks {
		if task.SprintID == nil {
			t.Fatalf("expected task %s to be assigned to imported sprint", task.ID)
		}
	}

	var comments int64
	if err := db.Model(&model.PMComment{}).Count(&comments).Error; err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if comments != 1 {
		t.Fatalf("expected 1 stored comment, got %d", comments)
	}
	var taskLinks int64
	if err := db.Model(&model.PMTaskLink{}).Count(&taskLinks).Error; err != nil {
		t.Fatalf("count task links: %v", err)
	}
	if taskLinks != 1 {
		t.Fatalf("expected 1 stored task link, got %d", taskLinks)
	}
	if err := db.Model(&model.PMTask{}).Where("workspace_id = ?", workspaceID).Update("sprint_id", nil).Error; err != nil {
		t.Fatalf("clear imported task sprint assignment: %v", err)
	}

	secondResult, _, err := svc.executeShortcutAPIImport(context.Background(), workspaceID, adminID, req, "")
	if err != nil {
		t.Fatalf("execute Shortcut API import second run: %v", err)
	}
	if secondResult.TasksCreated != 0 || secondResult.TasksSkipped != 2 {
		t.Fatalf("expected API rerun to skip tasks, got created=%d skipped=%d", secondResult.TasksCreated, secondResult.TasksSkipped)
	}
	if secondResult.DocsCreated != 0 || secondResult.DocsSkipped != 1 {
		t.Fatalf("expected API rerun to skip Shortcut doc, got created=%d skipped=%d", secondResult.DocsCreated, secondResult.DocsSkipped)
	}
	if err := db.Model(&model.PMComment{}).Count(&comments).Error; err != nil {
		t.Fatalf("count comments after rerun: %v", err)
	}
	if comments != 1 {
		t.Fatalf("expected comment import to be idempotent, got %d comments", comments)
	}
	if err := db.Where("workspace_id = ?", workspaceID).Find(&tasks).Error; err != nil {
		t.Fatalf("reload imported tasks after rerun: %v", err)
	}
	for _, task := range tasks {
		if task.SprintID == nil {
			t.Fatalf("expected rerun to repair sprint assignment for task %s", task.ID)
		}
	}
}

func TestPMImportServiceShortcutAPIUsesExplicitTeamMappingAndTeamWorkflow(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)
	targetTeam := model.WorkspaceTeam{ID: uuid.NewString(), WorkspaceID: workspaceID, Name: "Platform"}
	if err := db.Create(&targetTeam).Error; err != nil {
		t.Fatalf("seed target team: %v", err)
	}
	handler := newShortcutAPITestHandler(t)

	prevBaseURL := shortcutAPIBaseURL
	prevHTTPClientFactory := shortcutHTTPClientFactory
	shortcutAPIBaseURL = "https://shortcut.test"
	shortcutHTTPClientFactory = func() *http.Client {
		return &http.Client{Transport: handlerRoundTripper{handler: handler}}
	}
	t.Cleanup(func() {
		shortcutAPIBaseURL = prevBaseURL
		shortcutHTTPClientFactory = prevHTTPClientFactory
	})

	req := model.ShortcutAPIImportExecuteRequest{
		APIToken: "test-token",
		UserMappings: map[string]string{
			"owner.one@example.com": "user-owner-one",
			"owner.two@example.com": "user-owner-two",
		},
		TeamMappings: map[string]string{
			"Dev Team": "existing:" + targetTeam.ID,
		},
		WorkflowStateMappings: shortcutAPIImportWorkflowMappings(),
		Options:               model.ShortcutImportOptions{ImportArchived: true, ImportCompleted: true},
	}
	result, _, err := svc.executeShortcutAPIImport(context.Background(), workspaceID, adminID, req, "")
	if err != nil {
		t.Fatalf("execute Shortcut API import with explicit team mapping: %v", err)
	}
	if result.TeamsCreated != 0 {
		t.Fatalf("expected no teams created when mapped to existing team, got %d", result.TeamsCreated)
	}
	if result.WorkflowsCreated != 1 {
		t.Fatalf("expected one team-scoped workflow, got %d", result.WorkflowsCreated)
	}

	var tasks []model.PMTask
	if err := db.Where("workspace_id = ?", workspaceID).Find(&tasks).Error; err != nil {
		t.Fatalf("load imported tasks: %v", err)
	}
	for _, task := range tasks {
		if task.TeamID == nil || *task.TeamID != targetTeam.ID {
			t.Fatalf("expected task %s to map to Platform team, got %v", task.ID, task.TeamID)
		}
	}

	var workflow model.PMWorkflow
	if err := db.Where("workspace_id = ? AND name = ?", workspaceID, "Product Development").First(&workflow).Error; err != nil {
		t.Fatalf("load imported workflow: %v", err)
	}
	if workflow.TeamID == nil || *workflow.TeamID != targetTeam.ID {
		t.Fatalf("expected imported workflow to be scoped to Platform team, got %v", workflow.TeamID)
	}

	var devTeamCount int64
	if err := db.Model(&model.WorkspaceTeam{}).Where("workspace_id = ? AND name = ?", workspaceID, "Dev Team").Count(&devTeamCount).Error; err != nil {
		t.Fatalf("count Dev Team rows: %v", err)
	}
	if devTeamCount != 0 {
		t.Fatalf("expected no auto-created Dev Team when explicit mapping exists, got %d", devTeamCount)
	}
}

func TestPMImportServiceShortcutAPIRerunRemapsExistingStoriesToExplicitTeam(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)
	targetTeam := model.WorkspaceTeam{ID: uuid.NewString(), WorkspaceID: workspaceID, Name: "Platform"}
	if err := db.Create(&targetTeam).Error; err != nil {
		t.Fatalf("seed target team: %v", err)
	}
	handler := newShortcutAPITestHandler(t)

	prevBaseURL := shortcutAPIBaseURL
	prevHTTPClientFactory := shortcutHTTPClientFactory
	shortcutAPIBaseURL = "https://shortcut.test"
	shortcutHTTPClientFactory = func() *http.Client {
		return &http.Client{Transport: handlerRoundTripper{handler: handler}}
	}
	t.Cleanup(func() {
		shortcutAPIBaseURL = prevBaseURL
		shortcutHTTPClientFactory = prevHTTPClientFactory
	})

	baseReq := model.ShortcutAPIImportExecuteRequest{
		APIToken: "test-token",
		UserMappings: map[string]string{
			"owner.one@example.com": "user-owner-one",
			"owner.two@example.com": "user-owner-two",
		},
		WorkflowStateMappings: shortcutAPIImportWorkflowMappings(),
		Options:               model.ShortcutImportOptions{ImportArchived: true, ImportCompleted: true},
	}
	firstResult, _, err := svc.executeShortcutAPIImport(context.Background(), workspaceID, adminID, baseReq, "")
	if err != nil {
		t.Fatalf("execute initial Shortcut API import: %v", err)
	}
	if firstResult.TeamsCreated != 1 || firstResult.TasksCreated != 2 {
		t.Fatalf("expected initial import to create Dev Team and 2 tasks, got teams=%d tasks=%d", firstResult.TeamsCreated, firstResult.TasksCreated)
	}

	mappedReq := baseReq
	mappedReq.TeamMappings = map[string]string{"Dev Team": "existing:" + targetTeam.ID}
	secondResult, _, err := svc.executeShortcutAPIImport(context.Background(), workspaceID, adminID, mappedReq, "")
	if err != nil {
		t.Fatalf("execute remapping Shortcut API import: %v", err)
	}
	if secondResult.TasksCreated != 0 || secondResult.TasksSkipped != 2 {
		t.Fatalf("expected rerun to skip existing tasks, got created=%d skipped=%d", secondResult.TasksCreated, secondResult.TasksSkipped)
	}

	var platformWorkflow model.PMWorkflow
	if err := db.Where("workspace_id = ? AND name = ? AND team_id = ?", workspaceID, "Product Development", targetTeam.ID).First(&platformWorkflow).Error; err != nil {
		t.Fatalf("load Platform workflow: %v", err)
	}
	var tasks []model.PMTask
	if err := db.Where("workspace_id = ?", workspaceID).Find(&tasks).Error; err != nil {
		t.Fatalf("load imported tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 imported tasks after rerun, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.TeamID == nil || *task.TeamID != targetTeam.ID {
			t.Fatalf("expected existing task %s to remap to Platform team, got %v", task.ID, task.TeamID)
		}
		if task.WorkflowID != platformWorkflow.ID {
			t.Fatalf("expected existing task %s to remap to Platform workflow, got %s", task.ID, task.WorkflowID)
		}
	}
}

func TestShortcutAPIClientListAllStoriesAppliesScopeAndMaxStories(t *testing.T) {
	var bodies []map[string]any
	detailFetches := map[string]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/stories/search", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode stories search body: %v", err)
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		if archived, _ := body["archived"].(bool); archived {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[
			{"id":1001,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"},
			{"id":1002,"created_at":"2026-02-01T00:00:00Z","updated_at":"2026-02-03T00:00:00Z"}
		]`))
	})
	mux.HandleFunc("/stories/1001", func(w http.ResponseWriter, r *http.Request) {
		detailFetches["1001"]++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1001,"name":"older","updated_at":"2026-01-02T00:00:00Z"}`))
	})
	mux.HandleFunc("/stories/1002", func(w http.ResponseWriter, r *http.Request) {
		detailFetches["1002"]++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1002,"name":"newer","updated_at":"2026-02-03T00:00:00Z"}`))
	})

	client := NewShortcutAPIClientWithBaseURL("test-token", "https://shortcut.test")
	client.client = &http.Client{Transport: handlerRoundTripper{handler: mux}}
	stories, total, err := client.ListAllStories(context.Background(), shortcutAPIStorySearchOptions{
		UpdatedAtStart: "2026-01-01T00:00:00Z",
		MaxStories:     1,
		SortField:      "updated_at",
	})
	if err != nil {
		t.Fatalf("list scoped stories: %v", err)
	}
	if total != 1 || len(stories) != 1 || stories[0].ID != 1002 {
		t.Fatalf("expected newest story only, total=%d stories=%+v", total, stories)
	}
	if detailFetches["1001"] != 0 || detailFetches["1002"] != 1 {
		t.Fatalf("expected only newest story detail fetch, got %+v", detailFetches)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected active and archived search bodies, got %d", len(bodies))
	}
	for _, body := range bodies {
		if body["updated_at_start"] != "2026-01-01T00:00:00Z" {
			t.Fatalf("expected updated_at_start in query body, got %+v", body)
		}
	}
}

func shortcutAPIImportWorkflowMappings() []model.ShortcutWorkflowStateMappingPayload {
	return []model.ShortcutWorkflowStateMappingPayload{
		{
			ShortcutWorkflowID:   "500",
			ShortcutWorkflowName: "Product Development",
			Mode:                 "create_new",
			NewWorkflowName:      "Product Development",
			States: []struct {
				ShortcutState   string `json:"shortcut_state"`
				NewStateName    string `json:"new_state_name,omitempty"`
				StateType       string `json:"state_type,omitempty"`
				Position        int    `json:"position,omitempty"`
				ExistingStateID string `json:"existing_state_id,omitempty"`
			}{
				{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
				{ShortcutState: "Done", NewStateName: "Done", StateType: model.PMStateTypeDone, Position: 1},
			},
		},
	}
}

func TestPMImportServiceExecuteShortcutUsesWorkflowIDsForStateMapping(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	csvData := shortcutImportCSVFromRows([]map[string]string{
		{
			"id": "2001", "name": "Alpha backlog", "type": "feature",
			"created_at": "2026/03/01 09:00:00", "updated_at": "2026/03/01 09:00:00", "utc_offset": "+00:00",
			"workflow": "Shared Workflow", "workflow_id": "wf-alpha", "state": "Backlog",
		},
		{
			"id": "2002", "name": "Beta in progress", "type": "feature",
			"created_at": "2026/03/01 09:00:00", "updated_at": "2026/03/01 09:00:00", "utc_offset": "+00:00",
			"workflow": "Shared Workflow", "workflow_id": "wf-beta", "state": "In Progress",
		},
	})

	preview, err := svc.PreviewShortcut(context.Background(), workspaceID, adminID, []byte(csvData), "")
	if err != nil {
		t.Fatalf("preview shortcut: %v", err)
	}
	if len(preview.Workflows) != 2 {
		t.Fatalf("expected 2 distinct workflow previews, got %d", len(preview.Workflows))
	}

	req := model.ShortcutImportExecuteRequest{
		WorkflowStateMappings: []model.ShortcutWorkflowStateMappingPayload{
			{
				ShortcutWorkflowID:   "wf-alpha",
				ShortcutWorkflowName: "Shared Workflow",
				Mode:                 "create_new",
				NewWorkflowName:      "Imported Alpha",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
				},
			},
			{
				ShortcutWorkflowID:   "wf-beta",
				ShortcutWorkflowName: "Shared Workflow",
				Mode:                 "create_new",
				NewWorkflowName:      "Imported Beta",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "In Progress", NewStateName: "In Progress", StateType: model.PMStateTypeStarted, Position: 0},
				},
			},
		},
		Options: model.ShortcutImportOptions{
			ImportArchived:  true,
			ImportCompleted: true,
		},
	}

	result, totalRows, err := svc.executeShortcutImport(context.Background(), workspaceID, adminID, []byte(csvData), req, "", "")
	if err != nil {
		t.Fatalf("execute shortcut import: %v", err)
	}
	if totalRows != 2 {
		t.Fatalf("expected 2 imported rows, got %d", totalRows)
	}
	if result.WorkflowsCreated != 2 {
		t.Fatalf("expected 2 workflows created, got %d", result.WorkflowsCreated)
	}

	var stories []model.PMTask
	if err := db.Where("workspace_id = ?", workspaceID).Order("external_id").Find(&stories).Error; err != nil {
		t.Fatalf("load stories: %v", err)
	}
	if len(stories) != 2 {
		t.Fatalf("expected 2 imported stories, got %d", len(stories))
	}
	if stories[0].WorkflowID == stories[1].WorkflowID {
		t.Fatal("expected same-named Shortcut workflows to map to different Helpin workflows")
	}

	for _, story := range stories {
		var state model.PMWorkflowState
		if err := db.Where("id = ?", story.WorkflowStateID).First(&state).Error; err != nil {
			t.Fatalf("load workflow state %s: %v", story.WorkflowStateID, err)
		}
		if state.WorkflowID != story.WorkflowID {
			t.Fatalf("story %s has state %s from workflow %s, expected workflow %s", story.ID, state.ID, state.WorkflowID, story.WorkflowID)
		}
		switch *story.ExternalID {
		case "2001":
			if state.Name != "Backlog" {
				t.Fatalf("expected story 2001 to map to Backlog, got %q", state.Name)
			}
		case "2002":
			if state.Name != "In Progress" {
				t.Fatalf("expected story 2002 to map to In Progress, got %q", state.Name)
			}
		default:
			t.Fatalf("unexpected story external id %q", *story.ExternalID)
		}
	}
}

func TestPMImportServiceExecuteShortcutAssignsImportedOwnersToTeams(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	pendingMember := model.WorkspaceMember{
		ID:          "wm-pending-owner",
		WorkspaceID: workspaceID,
		Email:       "pending.owner@example.com",
		DisplayName: "pending.owner@example.com",
		Role:        model.RoleMember,
		Status:      model.WorkspaceMemberStatusPending,
	}
	if err := db.Create(&pendingMember).Error; err != nil {
		t.Fatalf("seed pending workspace member: %v", err)
	}

	csvData := shortcutImportCSVFromRows([]map[string]string{
		{
			"id": "4001", "name": "Pending owner story", "type": "feature",
			"owners": "pending.owner@example.com", "team": "Growth",
			"created_at": "2026/03/01 09:00:00", "updated_at": "2026/03/01 09:00:00", "utc_offset": "+00:00",
			"workflow": "Product Development", "workflow_id": "500000199", "state": "Backlog",
		},
	})

	req := model.ShortcutImportExecuteRequest{
		WorkflowStateMappings: []model.ShortcutWorkflowStateMappingPayload{
			{
				ShortcutWorkflowName: "Product Development",
				Mode:                 "create_new",
				NewWorkflowName:      "Imported Product Development",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
				},
			},
		},
		Options: model.ShortcutImportOptions{
			ImportArchived:  true,
			ImportCompleted: true,
		},
	}

	if _, _, err := svc.executeShortcutImport(context.Background(), workspaceID, adminID, []byte(csvData), req, "", ""); err != nil {
		t.Fatalf("execute shortcut import: %v", err)
	}

	var team model.WorkspaceTeam
	if err := db.Where("workspace_id = ? AND name = ?", workspaceID, "Growth").First(&team).Error; err != nil {
		t.Fatalf("load imported team: %v", err)
	}

	var membership model.TeamWorkspaceMembership
	if err := db.Where("team_id = ? AND workspace_member_id = ?", team.ID, pendingMember.ID).First(&membership).Error; err != nil {
		t.Fatalf("expected pending owner to be assigned to team: %v", err)
	}
}

func TestPMImportServiceExecuteShortcutUsesManualUserMappingsForTeamMemberships(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	csvData := shortcutImportCSVFromRows([]map[string]string{
		{
			"id": "4002", "name": "Alias owner story", "type": "feature",
			"owners": "alias.owner@example.com", "team": "Growth",
			"created_at": "2026/03/01 09:00:00", "updated_at": "2026/03/01 09:00:00", "utc_offset": "+00:00",
			"workflow": "Product Development", "workflow_id": "500000199", "state": "Backlog",
		},
	})

	req := model.ShortcutImportExecuteRequest{
		UserMappings: map[string]string{
			"alias.owner@example.com": "user-owner-one",
		},
		WorkflowStateMappings: []model.ShortcutWorkflowStateMappingPayload{
			{
				ShortcutWorkflowName: "Product Development",
				Mode:                 "create_new",
				NewWorkflowName:      "Imported Product Development",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
				},
			},
		},
		Options: model.ShortcutImportOptions{
			ImportArchived:  true,
			ImportCompleted: true,
		},
	}

	if _, _, err := svc.executeShortcutImport(context.Background(), workspaceID, adminID, []byte(csvData), req, "", ""); err != nil {
		t.Fatalf("execute shortcut import: %v", err)
	}

	var team model.WorkspaceTeam
	if err := db.Where("workspace_id = ? AND name = ?", workspaceID, "Growth").First(&team).Error; err != nil {
		t.Fatalf("load imported team: %v", err)
	}

	var workspaceMember model.WorkspaceMember
	if err := db.Where("workspace_id = ? AND user_id = ?", workspaceID, "user-owner-one").First(&workspaceMember).Error; err != nil {
		t.Fatalf("load workspace member for mapped user: %v", err)
	}

	var membership model.TeamWorkspaceMembership
	if err := db.Where("team_id = ? AND workspace_member_id = ?", team.ID, workspaceMember.ID).First(&membership).Error; err != nil {
		t.Fatalf("expected manually mapped owner to be assigned to team: %v", err)
	}

	var story model.PMTask
	if err := db.Where("workspace_id = ? AND external_id = ?", workspaceID, "4002").First(&story).Error; err != nil {
		t.Fatalf("load imported story: %v", err)
	}
	var ownerLinks int64
	if err := db.Table("pm_task_owners").Where("task_id = ? AND user_id = ?", story.ID, "user-owner-one").Count(&ownerLinks).Error; err != nil {
		t.Fatalf("count imported story owners: %v", err)
	}
	if ownerLinks != 1 {
		t.Fatalf("expected imported story owner link, got %d", ownerLinks)
	}
}

func TestPMImportServiceRewriteShortcutMediaBody(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	fakeAttachments := &fakeShortcutImportedAttachmentService{
		publicURLPrefix: "https://cdn.example.com/imported/",
	}
	fakeDownloader := &fakeShortcutMediaDownloader{
		mediaByURL: map[string]shortcutDownloadedMedia{
			"https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png": {
				FileName:    "downloaded-image.png",
				ContentType: "image/png",
				Data:        []byte("png-bytes"),
			},
		},
	}
	svc.attachmentService = fakeAttachments
	svc.mediaDownloader = fakeDownloader

	body := `<p>Screenshot</p><p><img src="https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png" alt="image.png"></p><p><a href="https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png">open</a></p>`
	rewritten, created, warnings := svc.rewriteShortcutMediaBody(context.Background(), workspaceID, adminID, "story", "story-123", body, "shortcut-token")

	if created != 1 {
		t.Fatalf("expected 1 imported attachment, got %d (warnings=%v rewritten=%s calls=%v)", created, warnings, rewritten, fakeDownloader.calls)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if !strings.Contains(rewritten, `src="https://cdn.example.com/imported/attachment-1/image.png"`) {
		t.Fatalf("expected rewritten img src, got %s", rewritten)
	}
	if !strings.Contains(rewritten, `href="https://cdn.example.com/imported/attachment-1/image.png"`) {
		t.Fatalf("expected rewritten href, got %s", rewritten)
	}
	if fakeDownloader.calls["https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png"] != 1 {
		t.Fatalf("expected a single media download, got %d", fakeDownloader.calls["https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png"])
	}
	if len(fakeAttachments.records) != 1 {
		t.Fatalf("expected 1 attachment record, got %d", len(fakeAttachments.records))
	}
	if fakeAttachments.records[0].req.FileName != "image.png" {
		t.Fatalf("expected imported filename image.png, got %q", fakeAttachments.records[0].req.FileName)
	}
	if string(fakeAttachments.records[0].data) != "png-bytes" {
		t.Fatalf("expected uploaded bytes to match downloader payload, got %q", string(fakeAttachments.records[0].data))
	}
}

func TestPMImportServiceRewriteShortcutMediaBodyPreservesFailedMediaAsVisibleLink(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	fakeAttachments := &fakeShortcutImportedAttachmentService{
		publicURLPrefix: "https://cdn.example.com/imported/",
	}
	fakeDownloader := &fakeShortcutMediaDownloader{
		mediaByURL: map[string]shortcutDownloadedMedia{},
	}
	svc.attachmentService = fakeAttachments
	svc.mediaDownloader = fakeDownloader

	body := `<p>Video</p><p><img src="https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/demo.mp4" alt="demo.mp4"></p><p><a href="https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/demo.mp4">open</a></p>`
	rewritten, created, warnings := svc.rewriteShortcutMediaBody(context.Background(), workspaceID, adminID, "story", "story-123", body, "shortcut-token")

	if created != 0 {
		t.Fatalf("expected no attachment for failed media, got %d", created)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %v", warnings)
	}
	if !strings.Contains(rewritten, `href="https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/demo.mp4"`) {
		t.Fatalf("expected original Shortcut URL to remain as link, got %s", rewritten)
	}
	if strings.Contains(rewritten, `<img`) {
		t.Fatalf("expected failed image embed to become a visible link, got %s", rewritten)
	}
	if !strings.Contains(rewritten, "Shortcut media not copied: demo.mp4") {
		t.Fatalf("expected visible failed-media label, got %s", rewritten)
	}
}

func TestPMImportServiceRewriteShortcutMediaTextPreservesFailedMarkdownMediaAsLink(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	fakeAttachments := &fakeShortcutImportedAttachmentService{
		publicURLPrefix: "https://cdn.example.com/imported/",
	}
	fakeDownloader := &fakeShortcutMediaDownloader{
		mediaByURL: map[string]shortcutDownloadedMedia{},
	}
	svc.attachmentService = fakeAttachments
	svc.mediaDownloader = fakeDownloader

	rawURL := "https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/demo.webm"
	text := "Review this recording ![demo.webm](" + rawURL + ")"
	rewritten, created, warnings := svc.rewriteShortcutMediaText(context.Background(), workspaceID, adminID, "task", "story-123", text, "shortcut-token")

	if created != 0 {
		t.Fatalf("expected no attachment for failed media, got %d", created)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %v", warnings)
	}
	if strings.Contains(rewritten, "![demo.webm]") {
		t.Fatalf("expected failed image markdown to become a normal link, got %s", rewritten)
	}
	if !strings.Contains(rewritten, "[Shortcut media not copied: demo.webm") || !strings.Contains(rewritten, "]("+rawURL+")") {
		t.Fatalf("expected visible fallback markdown link, got %s", rewritten)
	}
}

func TestPMImportServiceImportShortcutStoryMediaUpdatesDescriptions(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	fakeAttachments := &fakeShortcutImportedAttachmentService{
		publicURLPrefix: "https://cdn.example.com/imported/",
	}
	fakeDownloader := &fakeShortcutMediaDownloader{
		mediaByURL: map[string]shortcutDownloadedMedia{
			"https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png": {
				FileName:    "downloaded-image.png",
				ContentType: "image/png",
				Data:        []byte("png-bytes"),
			},
		},
	}
	svc.attachmentService = fakeAttachments
	svc.mediaDownloader = fakeDownloader

	req := model.ShortcutImportExecuteRequest{
		UserMappings: map[string]string{
			"owner.one@example.com": "user-owner-one",
			"owner.two@example.com": "user-owner-two",
		},
		WorkflowStateMappings: []model.ShortcutWorkflowStateMappingPayload{
			{
				ShortcutWorkflowName: "Product Development",
				Mode:                 "create_new",
				NewWorkflowName:      "Imported Product Development",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
					{ShortcutState: "Completed", NewStateName: "Completed", StateType: model.PMStateTypeDone, Position: 1},
				},
			},
		},
		Options: model.ShortcutImportOptions{
			ImportArchived:  true,
			ImportCompleted: true,
		},
	}

	if _, _, err := svc.executeShortcutImport(context.Background(), workspaceID, adminID, []byte(shortcutImportTestCSV()), req, "", ""); err != nil {
		t.Fatalf("execute shortcut import: %v", err)
	}

	var story model.PMTask
	if err := db.Where("workspace_id = ? AND external_id = ?", workspaceID, "113165").First(&story).Error; err != nil {
		t.Fatalf("load imported story: %v", err)
	}
	if story.Description == nil || !strings.Contains(*story.Description, "media.app.shortcut.com") {
		t.Fatalf("expected imported story to still contain Shortcut media before migration, got %v", story.Description)
	}

	attachmentsCreated, warnings := svc.importShortcutStoryMedia(context.Background(), workspaceID, adminID, []string{story.ID}, "shortcut-token", "", 1, 1)
	if attachmentsCreated != 1 {
		t.Fatalf("expected 1 imported attachment, got %d (warnings=%v calls=%v)", attachmentsCreated, warnings, fakeDownloader.calls)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if err := db.Where("id = ?", story.ID).First(&story).Error; err != nil {
		t.Fatalf("reload imported story: %v", err)
	}
	if story.Description == nil {
		t.Fatal("expected migrated story description")
	}
	if strings.Contains(*story.Description, "media.app.shortcut.com") {
		t.Fatalf("expected Shortcut media URL to be replaced, got %s", *story.Description)
	}
	if !strings.Contains(*story.Description, "https://cdn.example.com/imported/attachment-1/image.png") {
		t.Fatalf("expected migrated story description to use local media URL, got %s", *story.Description)
	}
}

func TestPMImportServiceImportShortcutStoryMediaUpdatesChecklistItems(t *testing.T) {
	db := newImportTestDB(t)
	svc, workspaceID, adminID := newImportTestService(t, db)

	fakeAttachments := &fakeShortcutImportedAttachmentService{
		publicURLPrefix: "https://cdn.example.com/imported/",
	}
	fakeDownloader := &fakeShortcutMediaDownloader{
		mediaByURL: map[string]shortcutDownloadedMedia{
			"https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/task-image.png": {
				FileName:    "downloaded-task-image.png",
				ContentType: "image/png",
				Data:        []byte("task-png-bytes"),
			},
		},
	}
	svc.attachmentService = fakeAttachments
	svc.mediaDownloader = fakeDownloader

	csvData := shortcutImportCSVFromRows([]map[string]string{
		{
			"id": "3001", "name": "Checklist media story", "type": "feature", "requester": "admin@example.com",
			"created_at": "2026/03/05 10:00:00", "updated_at": "2026/03/05 10:00:00", "utc_offset": "+00:00",
			"workflow": "Product Development", "workflow_id": "500000199", "state": "Backlog",
			"tasks": "[ ] Review screenshot ![task-image.png](https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/task-image.png)",
		},
	})

	req := model.ShortcutImportExecuteRequest{
		WorkflowStateMappings: []model.ShortcutWorkflowStateMappingPayload{
			{
				ShortcutWorkflowName: "Product Development",
				Mode:                 "create_new",
				NewWorkflowName:      "Imported Product Development",
				States: []struct {
					ShortcutState   string `json:"shortcut_state"`
					NewStateName    string `json:"new_state_name,omitempty"`
					StateType       string `json:"state_type,omitempty"`
					Position        int    `json:"position,omitempty"`
					ExistingStateID string `json:"existing_state_id,omitempty"`
				}{
					{ShortcutState: "Backlog", NewStateName: "Backlog", StateType: model.PMStateTypeBacklog, Position: 0},
				},
			},
		},
		Options: model.ShortcutImportOptions{
			ImportArchived:  true,
			ImportCompleted: true,
		},
	}

	if _, _, err := svc.executeShortcutImport(context.Background(), workspaceID, adminID, []byte(csvData), req, "", ""); err != nil {
		t.Fatalf("execute shortcut import: %v", err)
	}

	var story model.PMTask
	if err := db.Where("workspace_id = ? AND external_id = ?", workspaceID, "3001").First(&story).Error; err != nil {
		t.Fatalf("load imported story: %v", err)
	}

	var checklistItem model.PMChecklistItem
	if err := db.Where("task_id = ?", story.ID).First(&checklistItem).Error; err != nil {
		t.Fatalf("load imported checklist item: %v", err)
	}
	if !strings.Contains(checklistItem.Text, "media.app.shortcut.com") {
		t.Fatalf("expected checklist item to still contain Shortcut media before migration, got %s", checklistItem.Text)
	}

	attachmentsCreated, warnings := svc.importShortcutStoryMedia(context.Background(), workspaceID, adminID, []string{story.ID}, "shortcut-token", "", 1, 1)
	if attachmentsCreated != 1 {
		t.Fatalf("expected 1 imported attachment, got %d (warnings=%v calls=%v)", attachmentsCreated, warnings, fakeDownloader.calls)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if err := db.Where("id = ?", checklistItem.ID).First(&checklistItem).Error; err != nil {
		t.Fatalf("reload imported checklist item: %v", err)
	}
	if strings.Contains(checklistItem.Text, "media.app.shortcut.com") {
		t.Fatalf("expected Shortcut media URL in checklist item to be replaced, got %s", checklistItem.Text)
	}
	if !strings.Contains(checklistItem.Text, "https://cdn.example.com/imported/attachment-1/task-image.png") {
		t.Fatalf("expected checklist item to use local media URL, got %s", checklistItem.Text)
	}
}

func newImportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	registerTestUUIDCallback(t, db)
	createImportTestSchema(t, db)
	return db
}

type fakeShortcutImportedAttachmentService struct {
	publicURLPrefix string
	records         []fakeImportedAttachmentRecord
}

type fakeImportedAttachmentRecord struct {
	req         model.CreateAttachmentRequest
	workspaceID string
	userID      string
	data        []byte
}

func (f *fakeShortcutImportedAttachmentService) SupportsPublicURL() bool {
	return true
}

func (f *fakeShortcutImportedAttachmentService) CreateImported(ctx context.Context, req model.CreateAttachmentRequest, workspaceID, userID string, body io.Reader) (*model.AttachmentResponse, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	f.records = append(f.records, fakeImportedAttachmentRecord{
		req:         req,
		workspaceID: workspaceID,
		userID:      userID,
		data:        data,
	})
	id := fmt.Sprintf("attachment-%d", len(f.records))
	return &model.AttachmentResponse{
		Attachment: model.PMAttachment{
			ID:           id,
			WorkspaceID:  workspaceID,
			EntityType:   req.EntityType,
			EntityID:     req.EntityID,
			FileName:     req.FileName,
			FileSize:     req.FileSize,
			ContentType:  req.ContentType,
			StorageKey:   "imported/" + id,
			IsUploaded:   true,
			UploadedByID: userID,
		},
		PublicURL: f.publicURLPrefix + id + "/" + req.FileName,
	}, nil
}

type fakeShortcutMediaDownloader struct {
	mediaByURL map[string]shortcutDownloadedMedia
	calls      map[string]int
}

func (f *fakeShortcutMediaDownloader) Download(ctx context.Context, rawURL, apiToken string) (*shortcutDownloadedMedia, error) {
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[rawURL]++
	media, ok := f.mediaByURL[rawURL]
	if !ok {
		return nil, fmt.Errorf("unexpected media URL %s", rawURL)
	}
	copy := media
	copy.Data = append([]byte(nil), media.Data...)
	return &copy, nil
}

func createImportTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			email_verified_at DATETIME,
			google_subject TEXT,
			avatar_url TEXT,
			avatar_style TEXT,
			avatar_seed TEXT,
			avatar_background_mode TEXT,
			avatar_background_color TEXT,
			default_workspace_id TEXT,
			totp_secret_encrypted TEXT,
			totp_verified BOOLEAN NOT NULL DEFAULT 0,
			recovery_codes_encrypted TEXT,
			is_platform_admin BOOLEAN NOT NULL DEFAULT 0,
			is_server_admin BOOLEAN NOT NULL DEFAULT 0,
			signup_verification_pending BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			workspace_key TEXT,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			description TEXT,
			company_product_context TEXT,
			website_url TEXT,
			logo_url TEXT,
			timezone TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_members (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			user_id TEXT,
			email TEXT NOT NULL,
			display_name TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			invited_by TEXT,
			invited_at DATETIME,
			accepted_at DATETIME,
			support_default_team_id TEXT,
			support_task_dialog_dismissed BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_teams (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			handle TEXT,
			description TEXT,
			manager_id TEXT,
			team_type TEXT NOT NULL DEFAULT 'engineering',
			default_task_type TEXT NOT NULL DEFAULT 'feature',
			docs_publisher_enabled BOOLEAN NOT NULL DEFAULT 0,
			sprints_enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL DEFAULT 'workspace_wide',
			type TEXT NOT NULL DEFAULT 'internal',
			default_review_days INTEGER,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			visibility TEXT NOT NULL DEFAULT 'workspace_wide',
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			sort_key TEXT NOT NULL DEFAULT '~',
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content JSON,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			import_source_html TEXT,
			import_source_system TEXT,
			import_source_object_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE team_workspace_memberships (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE (team_id, workspace_member_id)
		)`,
		`CREATE TABLE pm_workflows (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			team_id TEXT,
			default_state_id TEXT,
			auto_assign_owner BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			description TEXT,
			w_ip_limit INTEGER,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_workflow_states (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			description TEXT,
			color TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			objective_type TEXT NOT NULL,
			state TEXT NOT NULL,
			planned_start_date DATETIME,
			deadline DATETIME,
			health TEXT NOT NULL,
			health_comment TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL,
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			orchestrator_agent_id TEXT,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			active_planning_session_id TEXT,
			active_flow_run_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (epic_id, objective_id)
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (epic_id, label_id)
		)`,
		`CREATE TABLE pm_sprints (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			start_date DATETIME,
			end_date DATETIME,
			team_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			task_type TEXT NOT NULL,
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL,
			severity TEXT NOT NULL,
			deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			plan_document_id TEXT,
			template_id TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_task_owners (
			task_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (task_id, user_id)
		)`,
		`CREATE TABLE pm_task_labels (
			task_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (task_id, label_id)
		)`,
		`CREATE TABLE pm_checklist_items (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			text TEXT NOT NULL,
			completed BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			assignee_id TEXT,
			due_date DATE,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_comments (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			author_id TEXT NOT NULL,
			agent_id TEXT,
			agent_name TEXT,
			agent_run_id TEXT,
			body TEXT NOT NULL,
			parent_id TEXT,
			block_id TEXT,
			block_range TEXT,
			anchor_text TEXT,
			resolved_at DATETIME,
			resolved_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_external_links (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			created_by_id TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_task_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			source_task_id TEXT NOT NULL,
			target_task_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_import_jobs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			file_name TEXT NOT NULL,
			total_rows INTEGER NOT NULL DEFAULT 0,
			progress INTEGER NOT NULL DEFAULT 0,
			current_step TEXT,
			steps_completed INTEGER NOT NULL DEFAULT 0,
			steps_total INTEGER NOT NULL DEFAULT 0,
			entities_processed INTEGER NOT NULL DEFAULT 0,
			entities_total INTEGER NOT NULL DEFAULT 0,
			result TEXT,
			error TEXT,
			payload_encrypted TEXT,
			workflow_id TEXT,
			started_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			completed_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
}

func registerTestUUIDCallback(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Callback().Create().Before("gorm:before_create").Register("test:assign_uuid", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		field := tx.Statement.Schema.LookUpField("ID")
		if field == nil || field.FieldType.Kind() != reflect.String {
			return
		}

		assignID := func(value reflect.Value) {
			if _, zero := field.ValueOf(tx.Statement.Context, value); !zero {
				return
			}
			_ = field.Set(tx.Statement.Context, value, uuid.NewString())
		}

		switch tx.Statement.ReflectValue.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < tx.Statement.ReflectValue.Len(); i++ {
				assignID(tx.Statement.ReflectValue.Index(i))
			}
		default:
			assignID(tx.Statement.ReflectValue)
		}
	})
	if err != nil {
		t.Fatalf("register uuid callback: %v", err)
	}
}

func newImportTestService(t *testing.T, db *gorm.DB) (*PMImportService, string, string) {
	t.Helper()
	workspaceID := "ws-import"
	adminID := "user-admin"
	users := []model.User{
		{ID: adminID, Email: "admin@example.com", PasswordHash: "x", FullName: "Admin User"},
		{ID: "user-requester", Email: "requester@example.com", PasswordHash: "x", FullName: "Riley Requester"},
		{ID: "user-owner-one", Email: "owner.one@example.com", PasswordHash: "x", FullName: "Owner One"},
		{ID: "user-owner-two", Email: "owner.two@example.com", PasswordHash: "x", FullName: "Owner Two"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	workspace := model.Workspace{
		ID:        workspaceID,
		Name:      "Import Workspace",
		Slug:      "import-workspace",
		OwnerID:   adminID,
		Timezone:  "UTC",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.Create(&workspace).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	memberships := []model.WorkspaceMember{
		{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: stringPtr(adminID), Email: "admin@example.com", DisplayName: "Admin User", Role: model.RoleOwner, Status: model.WorkspaceMemberStatusActive},
		{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: stringPtr("user-requester"), Email: "requester@example.com", DisplayName: "Riley Requester", Role: model.RoleMember, Status: model.WorkspaceMemberStatusActive},
		{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: stringPtr("user-owner-one"), Email: "owner.one@example.com", DisplayName: "Owner One", Role: model.RoleMember, Status: model.WorkspaceMemberStatusActive},
		{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: stringPtr("user-owner-two"), Email: "owner.two@example.com", DisplayName: "Owner Two", Role: model.RoleMember, Status: model.WorkspaceMemberStatusActive},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("seed workspace members: %v", err)
	}

	return NewPMImportService(db, repository.NewWorkspaceRepository(db), repository.NewPMWorkflowRepository(db), nil), workspaceID, adminID
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (h handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func newShortcutAPITestHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(value); err != nil {
			t.Fatalf("encode Shortcut API fixture: %v", err)
		}
	}
	requireToken := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Shortcut-Token") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return false
		}
		return true
	}

	mux.HandleFunc("/member", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, map[string]any{
			"id": "member-admin",
			"workspace2": map[string]any{
				"id":                  "shortcut-workspace",
				"name":                "Shortcut Workspace",
				"url_slug":            "shortcut-workspace",
				"default_workflow_id": 500,
			},
		})
	})
	mux.HandleFunc("/members", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{
			{"id": "member-requester", "profile": map[string]any{"email_address": "requester@example.com", "name": "Riley Requester"}},
			{"id": "member-owner-one", "profile": map[string]any{"email_address": "owner.one@example.com", "name": "Owner One"}},
			{"id": "member-owner-two", "profile": map[string]any{"email_address": "owner.two@example.com", "name": "Owner Two"}},
		})
	})
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{
			"id":   500,
			"name": "Product Development",
			"states": []map[string]any{
				{"id": 10, "name": "Backlog", "type": "backlog", "position": 0},
				{"id": 20, "name": "Done", "type": "done", "position": 1},
			},
		}})
	})
	mux.HandleFunc("/labels", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{"id": 700, "name": "frontend", "color": "#3b82f6"}})
	})
	mux.HandleFunc("/groups", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{"id": "group-dev", "name": "Dev Team", "mention_name": "dev"}})
	})
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{})
	})
	mux.HandleFunc("/documents", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{
			"id":      "11111111-1111-1111-1111-111111111111",
			"title":   "Shortcut Launch Plan",
			"app_url": "https://app.shortcut.com/acme/doc/11111111-1111-1111-1111-111111111111",
		}})
	})
	mux.HandleFunc("/documents/11111111-1111-1111-1111-111111111111", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		if r.URL.Query().Get("content_format") != "html" {
			http.Error(w, "expected content_format=html", http.StatusBadRequest)
			return
		}
		write(w, map[string]any{
			"id":               "11111111-1111-1111-1111-111111111111",
			"title":            "Shortcut Launch Plan",
			"app_url":          "https://app.shortcut.com/acme/doc/11111111-1111-1111-1111-111111111111",
			"content_html":     "<h1>Launch Plan</h1><p>Imported doc body.</p>",
			"content_markdown": "# Launch Plan\n\nImported doc body.",
			"created_at":       "2026-01-01T00:00:00Z",
			"updated_at":       "2026-01-02T00:00:00Z",
		})
	})
	mux.HandleFunc("/iterations", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{
			"id":         400,
			"name":       "Sprint 1",
			"start_date": "2026-01-01T00:00:00Z",
			"end_date":   "2026-01-14T00:00:00Z",
			"status":     "done",
			"group_ids":  []string{"group-dev"},
		}})
	})
	mux.HandleFunc("/iterations/400/stories", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		if r.URL.RawQuery != "" {
			http.Error(w, "iteration stories endpoint should not include query params", http.StatusBadRequest)
			return
		}
		write(w, []map[string]any{{"id": 1001}, {"id": 1002}})
	})
	mux.HandleFunc("/objectives", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{
			"id":          300,
			"name":        "Q1 Launch",
			"description": "Launch objective",
			"started":     true,
			"created_at":  "2026-01-01T00:00:00Z",
			"updated_at":  "2026-01-02T00:00:00Z",
		}})
	})
	mux.HandleFunc("/epics", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{
			"id":                 200,
			"name":               "Import Epic",
			"description":        "Epic description",
			"started":            true,
			"objective_ids":      []int{300},
			"group_id":           "group-dev",
			"planned_start_date": "2026-01-01T00:00:00Z",
			"deadline":           "2026-02-01T00:00:00Z",
			"created_at":         "2026-01-01T00:00:00Z",
			"updated_at":         "2026-01-02T00:00:00Z",
			"labels":             []map[string]any{{"id": 700, "name": "frontend", "color": "#3b82f6"}},
		}})
	})
	mux.HandleFunc("/stories/search", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if archived, _ := body["archived"].(bool); archived {
			write(w, []map[string]any{})
			return
		}
		write(w, []map[string]any{{"id": 1001}, {"id": 1002}})
	})
	mux.HandleFunc("/search/stories", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("legacy /search/stories endpoint should not be used for full API imports")
	})
	mux.HandleFunc("/stories/1001", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, map[string]any{
			"id":                1001,
			"app_url":           "https://app.shortcut.com/acme/story/1001",
			"name":              "API Story One",
			"description":       "Imported from API",
			"story_type":        "feature",
			"workflow_id":       500,
			"workflow_state_id": 10,
			"requested_by_id":   "member-requester",
			"owner_ids":         []string{"member-owner-one"},
			"epic_id":           200,
			"group_id":          "group-dev",
			"labels":            []map[string]any{{"id": 700, "name": "frontend", "color": "#3b82f6"}},
			"tasks":             []map[string]any{{"id": 9001, "description": "Checklist from API", "complete": true, "position": 0}},
			"comments":          []map[string]any{{"id": 8001, "text": "Looks good", "author_id": "member-owner-one", "story_id": 1001, "created_at": "2026-01-03T00:00:00Z", "updated_at": "2026-01-03T00:00:00Z"}},
			"story_links":       []map[string]any{{"id": 6001, "subject_id": 1001, "object_id": 1002, "verb": "blocks"}},
			"external_links":    []string{"https://example.com/spec"},
			"files":             []map[string]any{{"id": 5001, "name": "design.png", "url": "https://files.example.com/design.png"}},
			"created_at":        "2026-01-03T00:00:00Z",
			"updated_at":        "2026-01-04T00:00:00Z",
		})
	})
	mux.HandleFunc("/stories/1002", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, map[string]any{
			"id":                1002,
			"app_url":           "https://app.shortcut.com/acme/story/1002",
			"name":              "API Story Two",
			"description":       "Second story",
			"story_type":        "bug",
			"workflow_id":       500,
			"workflow_state_id": 20,
			"requested_by_id":   "member-requester",
			"owner_ids":         []string{"member-owner-two"},
			"epic_id":           200,
			"group_id":          "group-dev",
			"completed":         true,
			"created_at":        "2026-01-05T00:00:00Z",
			"updated_at":        "2026-01-06T00:00:00Z",
			"completed_at":      "2026-01-06T00:00:00Z",
		})
	})
	mux.HandleFunc("/stories/1001/comments", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{{"id": 8001, "text": "Looks good", "author_id": "member-owner-one", "story_id": 1001, "created_at": "2026-01-03T00:00:00Z", "updated_at": "2026-01-03T00:00:00Z"}})
	})
	mux.HandleFunc("/stories/1002/comments", func(w http.ResponseWriter, r *http.Request) {
		if !requireToken(w, r) {
			return
		}
		write(w, []map[string]any{})
	})

	return mux
}

func shortcutImportTestCSV() string {
	rows := []map[string]string{
		{
			"id": "85463", "name": "Paid Ads Attribution Improvements", "type": "feature", "requester": "requester@example.com",
			"description": "Sticky totals", "is_completed": "true", "created_at": "2025/03/13 02:27:02", "started_at": "2025/03/17 13:49:40",
			"updated_at": "2025/03/26 23:35:41", "moved_at": "2025/03/26 23:35:41", "completed_at": "2025/03/26 23:35:41",
			"estimate": "3", "is_blocked": "false", "state": "Completed", "iteration_id": "84719", "iteration": "Dev Team: Week 4-5, 2026",
			"utc_offset": "+05:00", "is_archived": "false", "team": "Dev Team", "workflow": "Product Development", "workflow_id": "500000199",
			"priority": "High", "labels": "frontend",
		},
		{
			"id": "113165", "name": "Improve the report email subject", "type": "feature", "requester": "missing.requester@example.com",
			"owners": "owner.one@example.com;missing.owner@example.com", "description": strings.Join([]string{
				"**Email:** bod@hanzonation.com",
				"**Plan:** premium-monthly",
				"**Chat:** https://app.crisp.chat/website/e80c07ae-0687-4e09-b9dc-22ad3bdf27ff/inbox/session_c5c5c898-3776-4f93-b1af-5142bde046fc/",
				"**Query** Seems like suddnely all data from my system is not showing?",
				"",
				"![image.png](https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png)",
			}, "\n"), "is_completed": "false",
			"created_at": "2026/03/04 22:18:07", "updated_at": "2026/03/04 22:19:55", "moved_at": "2026/03/04 22:18:07",
			"labels": "feature-request;customer", "state": "Backlog", "epic_id": "107534", "epic": "Q1 - 2026 - Features and Bugs",
			"iteration_id": "44783", "iteration": "Nov 7 - Nov 21", "utc_offset": "+05:00", "is_archived": "true",
			"team": "Dev Team", "epic_state": "in progress", "epic_is_archived": "false", "epic_created_at": "2026/01/07 15:57:32",
			"epic_started_at": "2026/01/07 15:59:43", "objective_id": "107533", "objective": "Q1 - 2026", "objective_state": "to do",
			"objective_created_at": "2026/01/07 15:56:20", "objective_started_at": "2026/01/01 05:00:00", "objective_due_date": "2026/03/31 05:00:00",
			"epic_planned_start_date": "2026/01/01 05:00:00", "workflow": "Product Development", "workflow_id": "500000199",
			"priority": "Medium", "custom_fields": "Plan=premium",
		},
		{
			"id": "101202", "name": "Integrate pinecone into Usermaven", "type": "chore", "requester": "admin@example.com",
			"owners": "owner.two@example.com", "description": "Search integration", "is_completed": "false", "created_at": "2026/02/10 09:00:00",
			"updated_at": "2026/02/11 09:00:00", "labels": "backend", "state": "Backlog", "epic_id": "107534", "epic": "Q1 - 2026 - Features and Bugs",
			"iteration_id": "44783", "iteration": "Nov 7 - Nov 21", "utc_offset": "+00:00", "is_archived": "false", "team": "Founders",
			"epic_state": "in progress", "epic_is_archived": "false", "objective_id": "107533", "objective": "Q1 - 2026",
			"objective_state": "to do", "workflow": "Product Development", "workflow_id": "500000199", "priority": "Low",
			"tasks": "[ ] Unit tests;[ ] Integrate pinecone into Usermaven",
		},
		{
			"id": "45480", "name": "Webhook delete event", "type": "bug", "requester": "requester@example.com",
			"owners": "owner.one@example.com;owner.two@example.com", "description": "Webhook should trigger", "is_completed": "false",
			"created_at": "2025/04/01 10:00:00", "updated_at": "2025/04/02 10:00:00", "labels": "backend",
			"iteration_id": "99999", "iteration": "Unknown Sprint Name", "utc_offset": "+00:00", "is_archived": "false", "team": "",
			"workflow": "Product Development", "workflow_id": "500000199", "state": "Backlog", "severity": "Severity 1",
			"tasks": "[X] create eventsource;[ ] Generate a webhook",
		},
	}
	return shortcutImportCSVFromRows(rows)
}

func shortcutImportCSVFromRows(rows []map[string]string) string {
	header := []string{
		"id", "name", "type", "requester", "owners", "description", "is_completed", "created_at", "started_at", "updated_at",
		"moved_at", "completed_at", "estimate", "external_ticket_count", "external_tickets", "is_blocked", "is_a_blocker", "due_date",
		"labels", "epic_labels", "tasks", "state", "epic_id", "epic", "project_id", "project", "iteration_id", "iteration", "utc_offset",
		"is_archived", "team_id", "team", "epic_state", "epic_is_archived", "epic_created_at", "epic_started_at", "epic_due_date",
		"objective_id", "objective", "objective_state", "objective_created_at", "objective_started_at", "objective_due_date",
		"objective_categories", "epic_planned_start_date", "workflow", "workflow_id", "priority", "severity", "product_area",
		"skill_set", "technical_area", "custom_fields", "parent_story_id",
	}
	var b strings.Builder
	writer := csv.NewWriter(&b)
	if err := writer.Write(header); err != nil {
		panic(err)
	}
	for _, row := range rows {
		record := make([]string, len(header))
		for idx, column := range header {
			record[idx] = row[column]
		}
		if err := writer.Write(record); err != nil {
			panic(err)
		}
	}
	writer.Flush()
	return b.String()
}

func assertImportState(t *testing.T, db *gorm.DB, workspaceID string) {
	t.Helper()

	var stories []model.PMTask
	if err := db.Where("workspace_id = ?", workspaceID).Order("external_id").Find(&stories).Error; err != nil {
		t.Fatalf("load stories: %v", err)
	}
	if len(stories) != 4 {
		t.Fatalf("expected 4 imported stories, got %d", len(stories))
	}
	for _, story := range stories {
		var state model.PMWorkflowState
		if err := db.Where("id = ?", story.WorkflowStateID).First(&state).Error; err != nil {
			t.Fatalf("load workflow state for story %s: %v", story.ID, err)
		}
		if state.WorkflowID != story.WorkflowID {
			t.Fatalf("story %s has state %s from workflow %s, expected workflow %s", story.ID, state.ID, state.WorkflowID, story.WorkflowID)
		}
		if story.ExternalID == nil {
			t.Fatalf("expected story %s to have external_id", story.ID)
		}
		switch *story.ExternalID {
		case "85463":
			if state.Name != "Completed" {
				t.Fatalf("expected story 85463 to map to Completed, got %q", state.Name)
			}
		default:
			if state.Name != "Backlog" {
				t.Fatalf("expected story %s to map to Backlog, got %q", *story.ExternalID, state.Name)
			}
		}
	}
	var markdownStory *model.PMTask
	for i := range stories {
		if stories[i].ExternalID != nil && *stories[i].ExternalID == "113165" {
			markdownStory = &stories[i]
		}
	}
	if markdownStory == nil || markdownStory.Description == nil {
		t.Fatal("expected imported markdown story description")
	}
	if !strings.Contains(*markdownStory.Description, "<strong>Email:</strong>") {
		t.Fatalf("expected rendered bold markdown in story description, got %s", *markdownStory.Description)
	}
	if !strings.Contains(*markdownStory.Description, `<a href="https://app.crisp.chat/website/e80c07ae-0687-4e09-b9dc-22ad3bdf27ff/inbox/session_c5c5c898-3776-4f93-b1af-5142bde046fc/">`) {
		t.Fatalf("expected rendered link in story description, got %s", *markdownStory.Description)
	}
	if !strings.Contains(*markdownStory.Description, `src="https://media.app.shortcut.com/api/attachments/files/clubhouse-assets/example/image.png" alt="image.png"`) {
		t.Fatalf("expected rendered image in story description, got %s", *markdownStory.Description)
	}

	var sprints []model.PMSprint
	if err := db.Where("workspace_id = ?", workspaceID).Order("external_id").Find(&sprints).Error; err != nil {
		t.Fatalf("load sprints: %v", err)
	}
	if len(sprints) != 3 {
		t.Fatalf("expected 3 imported sprints, got %d", len(sprints))
	}

	var unknownSprint *model.PMSprint
	for i := range sprints {
		if sprints[i].ExternalID != nil && *sprints[i].ExternalID == "99999" {
			unknownSprint = &sprints[i]
		}
	}
	if unknownSprint == nil {
		t.Fatal("expected sprint with external_id 99999")
	}
	if unknownSprint.StartDate != nil || unknownSprint.EndDate != nil {
		t.Fatal("expected unknown sprint to have null dates")
	}
	if unknownSprint.TeamID != nil {
		t.Fatal("expected unknown sprint to have null team")
	}

	var sharedSprint *model.PMSprint
	for i := range sprints {
		if sprints[i].ExternalID != nil && *sprints[i].ExternalID == "44783" {
			sharedSprint = &sprints[i]
		}
	}
	if sharedSprint == nil || sharedSprint.StartDate == nil || sharedSprint.EndDate == nil {
		t.Fatal("expected parsed date sprint for iteration 44783")
	}
	if sharedSprint.TeamID != nil {
		t.Fatal("expected multi-team sprint to have null team")
	}

	var importedEpic model.PMEpic
	if err := db.Where("workspace_id = ? AND external_id = ?", workspaceID, "107534").First(&importedEpic).Error; err != nil {
		t.Fatalf("load imported epic: %v", err)
	}
	if importedEpic.TeamID != nil {
		t.Fatal("expected multi-team epic to have null team")
	}

	var labels []model.PMLabel
	if err := db.Where("workspace_id = ?", workspaceID).Find(&labels).Error; err != nil {
		t.Fatalf("load labels: %v", err)
	}
	if len(labels) != 4 {
		t.Fatalf("expected 4 imported labels, got %d", len(labels))
	}
	for _, label := range labels {
		if label.Color == nil || strings.TrimSpace(*label.Color) == "" {
			t.Fatalf("expected imported label %q to have a color", label.Name)
		}
	}

	var ownerLinks int64
	if err := db.Model(&model.PMTaskOwner{}).Count(&ownerLinks).Error; err != nil {
		t.Fatalf("count owner links: %v", err)
	}
	if ownerLinks != 4 {
		t.Fatalf("expected 4 story owners, got %d", ownerLinks)
	}

	var checklistCount int64
	if err := db.Model(&model.PMChecklistItem{}).Count(&checklistCount).Error; err != nil {
		t.Fatalf("count checklist items: %v", err)
	}
	if checklistCount != 4 {
		t.Fatalf("expected 4 checklist items, got %d", checklistCount)
	}
}

func assertWarningContains(t *testing.T, warnings []string, needle string) {
	t.Helper()
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return
		}
	}
	t.Fatalf("expected warning containing %q, got %v", needle, warnings)
}
