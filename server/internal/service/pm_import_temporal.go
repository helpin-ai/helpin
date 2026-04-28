package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"gorm.io/gorm"
)

type PMImportActivities struct {
	importService *PMImportService
}

func NewPMImportActivities(importService *PMImportService) *PMImportActivities {
	if importService == nil {
		return nil
	}
	return &PMImportActivities{importService: importService}
}

func (a *PMImportActivities) ExecuteShortcutAPIImportActivity(ctx context.Context, input temporalapp.ShortcutImportWorkflowInput) error {
	if a == nil || a.importService == nil {
		return fmt.Errorf("pm import activity service is not configured")
	}
	return a.importService.executeStoredShortcutAPIImport(ctx, strings.TrimSpace(input.ImportID))
}

func (s *PMImportService) executeStoredShortcutAPIImport(ctx context.Context, importID string) error {
	if strings.TrimSpace(importID) == "" {
		return fmt.Errorf("import_id is required")
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND source = ?", importID, model.PMImportSourceShortcut).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("Shortcut import job not found")
		}
		return fmt.Errorf("load Shortcut import job: %w", err)
	}
	if job.Status == model.PMImportStatusCompleted || job.Status == model.PMImportStatusCanceled {
		return nil
	}
	if len(s.encryptionKey) != 32 {
		s.markStoredShortcutImportFailed(ctx, job.ID, "Shortcut import encryption key is not configured")
		return nil
	}
	if job.PayloadEncrypted == nil || strings.TrimSpace(*job.PayloadEncrypted) == "" {
		s.markStoredShortcutImportFailed(ctx, job.ID, "Shortcut import payload is missing")
		return nil
	}
	raw, err := appcrypto.DecryptString(*job.PayloadEncrypted, s.encryptionKey)
	if err != nil {
		s.markStoredShortcutImportFailed(ctx, job.ID, "Shortcut import payload could not be decrypted")
		return nil
	}
	var req model.ShortcutAPIImportExecuteRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		s.markStoredShortcutImportFailed(ctx, job.ID, "Shortcut import payload is invalid")
		return nil
	}
	s.runShortcutAPIImportWithContext(ctx, job.ID, job.WorkspaceID, job.StartedBy, req)
	return nil
}

func (s *PMImportService) markStoredShortcutImportFailed(ctx context.Context, jobID, message string) {
	errText := strings.TrimSpace(message)
	if errText == "" {
		errText = "Shortcut import failed"
	}
	now := time.Now().UTC()
	_ = s.updateJob(ctx, jobID, map[string]interface{}{
		"status":       model.PMImportStatusFailed,
		"error":        &errText,
		"completed_at": &now,
		"updated_at":   now,
	})
}
