package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestShortcutImportDiagnosticsGroupsImportIssues(t *testing.T) {
	result := &model.ShortcutImportResult{
		TasksCreated:       12,
		AttachmentsCreated: 4,
		Warnings: []string{
			`Failed to download Shortcut media "https://media.app.shortcut.com/file.png": temporary network error`,
			`Failed to store Shortcut media "https://media.app.shortcut.com/video.mov": file type video/quicktime is not allowed`,
			"7 stories had unmapped requester emails — requester_id set to null",
			`Owner email 'owner@example.com' not mapped — 3 owner links skipped`,
			`5 stories in workflow "Engineering" used the default state because Shortcut state "Review" could not be mapped`,
		},
	}

	diag := shortcutImportDiagnostics(result)

	if len(diag.FailedMedia) != 2 {
		t.Fatalf("failed media count = %d, want 2", len(diag.FailedMedia))
	}
	if len(diag.RetryableFailures) != 1 || diag.RetryableFailures[0].Key != "https://media.app.shortcut.com/file.png" {
		t.Fatalf("retryable failures = %+v, want download failure", diag.RetryableFailures)
	}
	if len(diag.NonRetryableFailures) != 1 || diag.NonRetryableFailures[0].Key != "https://media.app.shortcut.com/video.mov" {
		t.Fatalf("non-retryable failures = %+v, want unsupported video failure", diag.NonRetryableFailures)
	}
	if len(diag.UnmappedMembers) != 2 {
		t.Fatalf("unmapped members = %d, want 2", len(diag.UnmappedMembers))
	}
	if len(diag.UnmappedStates) != 1 {
		t.Fatalf("unmapped states = %d, want 1", len(diag.UnmappedStates))
	}
	if len(diag.WarningGroups) == 0 {
		t.Fatal("expected warning groups")
	}
}

func TestShortcutImportRetryBlockedReasonRequiresStoredPayload(t *testing.T) {
	job := model.PMImportJob{Status: model.PMImportStatusFailed}
	reason := shortcutImportRetryBlockedReason(job, make([]byte, 32))

	if reason != "This import cannot be retried because no stored API payload is available" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestReconcileStaleShortcutPreviewScans(t *testing.T) {
	db := newImportTestDB(t)
	svc := &PMImportService{db: db}

	now := time.Now().UTC()
	stale := now.Add(-shortcutImportStaleAfter - time.Minute)
	fresh := now.Add(-time.Minute)
	const workspaceID = "ws-1"

	jobs := []model.PMImportJob{
		{
			ID:          "stale-pending",
			WorkspaceID: workspaceID,
			Source:      model.PMImportSourceShortcutAPIPreview,
			Status:      model.PMImportStatusPending,
			FileName:    "shortcut-api-preview",
			StartedBy:   "user-1",
			CreatedAt:   stale,
			UpdatedAt:   stale,
		},
		{
			ID:          "stale-scanning",
			WorkspaceID: workspaceID,
			Source:      model.PMImportSourceShortcutAPIPreview,
			Status:      model.PMImportStatusScanning,
			FileName:    "shortcut-api-preview",
			StartedBy:   "user-1",
			CreatedAt:   stale,
			UpdatedAt:   stale,
		},
		{
			ID:          "fresh-pending",
			WorkspaceID: workspaceID,
			Source:      model.PMImportSourceShortcutAPIPreview,
			Status:      model.PMImportStatusPending,
			FileName:    "shortcut-api-preview",
			StartedBy:   "user-1",
			CreatedAt:   fresh,
			UpdatedAt:   fresh,
		},
		{
			ID:          "stale-other-source",
			WorkspaceID: workspaceID,
			Source:      model.PMImportSourceShortcut,
			Status:      model.PMImportStatusPending,
			FileName:    "shortcut",
			StartedBy:   "user-1",
			CreatedAt:   stale,
			UpdatedAt:   stale,
		},
		{
			ID:          "stale-other-workspace",
			WorkspaceID: "ws-2",
			Source:      model.PMImportSourceShortcutAPIPreview,
			Status:      model.PMImportStatusPending,
			FileName:    "shortcut-api-preview",
			StartedBy:   "user-1",
			CreatedAt:   stale,
			UpdatedAt:   stale,
		},
	}
	for i := range jobs {
		if err := db.Create(&jobs[i]).Error; err != nil {
			t.Fatalf("seed job %s: %v", jobs[i].ID, err)
		}
	}

	if err := svc.reconcileStaleShortcutPreviewScans(context.Background(), workspaceID); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	wantStatuses := map[string]string{
		"stale-pending":         model.PMImportStatusFailed,
		"stale-scanning":        model.PMImportStatusFailed,
		"fresh-pending":         model.PMImportStatusPending,
		"stale-other-source":    model.PMImportStatusPending,
		"stale-other-workspace": model.PMImportStatusPending,
	}
	for id, want := range wantStatuses {
		var got model.PMImportJob
		if err := db.First(&got, "id = ?", id).Error; err != nil {
			t.Fatalf("load %s: %v", id, err)
		}
		if got.Status != want {
			t.Errorf("%s status = %q, want %q", id, got.Status, want)
		}
		if want == model.PMImportStatusFailed {
			if got.Error == nil || *got.Error == "" {
				t.Errorf("%s expected error message to be set", id)
			}
			if got.CompletedAt == nil {
				t.Errorf("%s expected completed_at to be set", id)
			}
			if got.CurrentStep != "failed" {
				t.Errorf("%s current_step = %q, want %q", id, got.CurrentStep, "failed")
			}
		}
	}
}
