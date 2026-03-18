package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
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
	if err := db.Create(&model.PMStory{
		ID:              uuid.NewString(),
		WorkspaceID:     workspaceID,
		DisplayID:       1,
		Name:            "Existing Story",
		StoryType:       model.PMStoryTypeFeature,
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

	if resp.Summary.TotalStories != 4 {
		t.Fatalf("expected 4 stories, got %d", resp.Summary.TotalStories)
	}
	if resp.Summary.DuplicateStories != 1 {
		t.Fatalf("expected 1 duplicate story, got %d", resp.Summary.DuplicateStories)
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
		if user.Email == "azhar@contentstudio.io" {
			requesterMatched = user.MatchedUserID != nil && *user.MatchedUserID != ""
		}
	}
	if !requesterMatched {
		t.Fatal("expected azhar requester to auto-match")
	}

	if len(resp.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(resp.Workflows))
	}
	if resp.Workflows[0].StoryCount != 4 {
		t.Fatalf("expected workflow story count 4, got %d", resp.Workflows[0].StoryCount)
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
	if result.WorkflowsCreated != 1 {
		t.Fatalf("expected 1 workflow created, got %d", result.WorkflowsCreated)
	}
	if result.WorkflowStatesCreated != 2 {
		t.Fatalf("expected 2 workflow states created, got %d", result.WorkflowStatesCreated)
	}
	if result.ObjectivesCreated != 1 || result.EpicsCreated != 1 || result.SprintsCreated != 3 {
		t.Fatalf("unexpected entity counts: objectives=%d epics=%d sprints=%d", result.ObjectivesCreated, result.EpicsCreated, result.SprintsCreated)
	}
	if result.StoriesCreated != 4 || result.StoriesSkipped != 0 {
		t.Fatalf("unexpected story counts: created=%d skipped=%d", result.StoriesCreated, result.StoriesSkipped)
	}
	if result.OwnerLinksCreated != 4 {
		t.Fatalf("expected 4 owner links created, got %d", result.OwnerLinksCreated)
	}
	if result.LabelLinksCreated != 5 {
		t.Fatalf("expected 5 story label links created, got %d", result.LabelLinksCreated)
	}
	if result.ChecklistItemsCreated != 4 {
		t.Fatalf("expected 4 checklist items created, got %d", result.ChecklistItemsCreated)
	}
	assertWarningContains(t, result.Warnings, "1 imported sprints had null dates because iteration names could not be parsed")
	assertWarningContains(t, result.Warnings, "1 stories had unmapped requester emails")
	assertWarningContains(t, result.Warnings, "Owner email 'missing.owner@example.com' not mapped")

	assertImportState(t, db, workspaceID)

	secondResult, _, err := svc.executeShortcutImport(context.Background(), workspaceID, "user-admin", []byte(shortcutImportTestCSV()), req, "", "")
	if err != nil {
		t.Fatalf("execute shortcut import second run: %v", err)
	}
	if secondResult.StoriesCreated != 0 {
		t.Fatalf("expected no new stories on second import, got %d", secondResult.StoriesCreated)
	}
	if secondResult.StoriesSkipped != 4 {
		t.Fatalf("expected 4 skipped stories on second import, got %d", secondResult.StoriesSkipped)
	}
	if secondResult.EpicsCreated != 0 || secondResult.ObjectivesCreated != 0 || secondResult.SprintsCreated != 0 {
		t.Fatalf("expected no new deduped entities on second import, got epics=%d objectives=%d sprints=%d", secondResult.EpicsCreated, secondResult.ObjectivesCreated, secondResult.SprintsCreated)
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

	var stories []model.PMStory
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

	var story model.PMStory
	if err := db.Where("workspace_id = ? AND external_id = ?", workspaceID, "4002").First(&story).Error; err != nil {
		t.Fatalf("load imported story: %v", err)
	}
	if story.OwnerMemberID == nil || *story.OwnerMemberID != workspaceMember.ID {
		t.Fatalf("expected owner_member_id to use mapped workspace member, got %v want %s", story.OwnerMemberID, workspaceMember.ID)
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

	var story model.PMStory
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

	var story model.PMStory
	if err := db.Where("workspace_id = ? AND external_id = ?", workspaceID, "3001").First(&story).Error; err != nil {
		t.Fatalf("load imported story: %v", err)
	}

	var checklistItem model.PMChecklistItem
	if err := db.Where("story_id = ?", story.ID).First(&checklistItem).Error; err != nil {
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
			avatar_url TEXT,
			default_workspace_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			description TEXT,
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
			default_story_type TEXT NOT NULL DEFAULT 'feature',
			docs_publisher_enabled BOOLEAN NOT NULL DEFAULT 0,
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
		`CREATE TABLE pm_stories (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			story_type TEXT NOT NULL,
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
			template_id TEXT,
			external_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_story_owners (
			story_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, user_id)
		)`,
		`CREATE TABLE pm_story_labels (
			story_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, label_id)
		)`,
		`CREATE TABLE pm_checklist_items (
			id TEXT PRIMARY KEY,
			story_id TEXT NOT NULL,
			text TEXT NOT NULL,
			completed BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			assignee_id TEXT,
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
		{ID: "user-requester", Email: "azhar@contentstudio.io", PasswordHash: "x", FullName: "Azhar K"},
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
		{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: stringPtr("user-requester"), Email: "azhar@contentstudio.io", DisplayName: "Azhar K", Role: model.RoleMember, Status: model.WorkspaceMemberStatusActive},
		{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: stringPtr("user-owner-one"), Email: "owner.one@example.com", DisplayName: "Owner One", Role: model.RoleMember, Status: model.WorkspaceMemberStatusActive},
		{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: stringPtr("user-owner-two"), Email: "owner.two@example.com", DisplayName: "Owner Two", Role: model.RoleMember, Status: model.WorkspaceMemberStatusActive},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("seed workspace members: %v", err)
	}

	return NewPMImportService(db, repository.NewWorkspaceRepository(db), repository.NewPMWorkflowRepository(db), nil), workspaceID, adminID
}

func shortcutImportTestCSV() string {
	rows := []map[string]string{
		{
			"id": "85463", "name": "Paid Ads Attribution Improvements", "type": "feature", "requester": "azhar@contentstudio.io",
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
			"id": "45480", "name": "Webhook delete event", "type": "bug", "requester": "azhar@contentstudio.io",
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

	var stories []model.PMStory
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
	var markdownStory *model.PMStory
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
	if err := db.Model(&model.PMStoryOwner{}).Count(&ownerLinks).Error; err != nil {
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
