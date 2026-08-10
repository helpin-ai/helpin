package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"go.temporal.io/sdk/activity"
)

// DocsImportActivities exposes durable docs-import operations to Temporal.
type DocsImportActivities struct {
	importService *DocsImportService
}

// NewDocsImportActivities creates the Temporal docs-import activity adapter.
func NewDocsImportActivities(importService *DocsImportService) *DocsImportActivities {
	if importService == nil {
		return nil
	}
	return &DocsImportActivities{importService: importService}
}

// ExecuteHelpScoutImportActivity resumes and executes a stored HelpScout import.
func (a *DocsImportActivities) ExecuteHelpScoutImportActivity(ctx context.Context, input temporalapp.DocsImportWorkflowInput) error {
	if a == nil || a.importService == nil {
		return fmt.Errorf("docs import activity service is not configured")
	}
	return a.importService.executeStoredHelpScoutImport(ctx, strings.TrimSpace(input.ImportID))
}

func (s *DocsImportService) executeStoredHelpScoutImport(ctx context.Context, importID string) error {
	if importID == "" {
		return fmt.Errorf("import_id is required")
	}
	job, err := s.importRepo.GetByID(ctx, importID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("docs import job not found")
	}
	if job.Status == model.DocsImportStatusDone || job.Status == model.DocsImportStatusFailed || job.Status == model.DocsImportStatusInterrupted {
		return nil
	}
	if len(s.encryptionKey) != 32 {
		s.failJob(ctx, job.ID, "Docs import encryption key is not configured")
		return nil
	}
	if job.PayloadEncrypted == nil || strings.TrimSpace(*job.PayloadEncrypted) == "" {
		s.failJob(ctx, job.ID, "Docs import payload is missing")
		return nil
	}
	raw, err := appcrypto.DecryptString(*job.PayloadEncrypted, s.encryptionKey)
	if err != nil {
		s.failJob(ctx, job.ID, "Docs import payload could not be decrypted")
		return nil
	}
	var req model.DocsImportStartRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		s.failJob(ctx, job.ID, "Docs import payload is invalid")
		return nil
	}
	if req.TargetSpaceID == nil || strings.TrimSpace(*req.TargetSpaceID) == "" {
		s.failJob(ctx, job.ID, "Docs import target space is missing")
		return nil
	}
	return s.runImport(ctx, job.ID, req.APIKey, req, *req.TargetSpaceID, job.WorkspaceID, job.StartedBy, func(processed int) {
		activity.RecordHeartbeat(ctx, processed)
	})
}
