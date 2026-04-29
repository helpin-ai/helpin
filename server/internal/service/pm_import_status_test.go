package service

import (
	"testing"

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
